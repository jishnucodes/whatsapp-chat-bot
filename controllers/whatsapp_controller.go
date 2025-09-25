// controllers/whatsapp_controller.go
package controllers

import (
	"bytes"
	"context"
	"encoding/json"

	// "errors"

	// "errors"
	"io"
	"strconv"
	"sync"
	"time"

	// "encoding/json"
	"fmt"
	"log"
	"net/http"
	// "net/http/httputil"
	"strings"

	"clinic-chatbot-backend/models"
	"clinic-chatbot-backend/services"

	"github.com/gin-gonic/gin"
)

type WhatsAppController struct {
	whatsappService *services.WhatsAppService
	chatbotService  *services.ChatbotService
}

func NewWhatsAppController(whatsappService *services.WhatsAppService, chatbotService *services.ChatbotService) *WhatsAppController {
	return &WhatsAppController{
		whatsappService: whatsappService,
		chatbotService:  chatbotService,
	}
}

type Post struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type AppointmentData struct {
	PatientID       int    `json:"patientId"`
	PatientCode     string `json:"patientCode"`
	PatientName     string `json:"patientName"`
	Address         string `json:"address"`
	PhoneNumber     string `json:"phoneNumber"`
	DateOfBirth     string `json:"dateOfBirth"`
	DepartmentID    uint   `json:"departmentId"`
	AppointmentDate string `json:"appointmentDate"`
	DoctorID        uint   `json:"doctorId"`
	DoctorName      string `json:"doctorName"`
	OnlineTempToken uint   `json:"onlineTempToken"`
	TimeSlot        string `json:"timeSlot"`
	Step            string `json:"step"`
	CreatedFrom     string `json:"createdFrom"`
}

var appointmentState = make(map[string]*AppointmentData) // userID → data

// Track user states
var (
	userState  = make(map[string]string) // userID -> state
	stateMutex sync.Mutex
)

// Appointment structure
// type Appointment struct {
//     ID     string
//     Doctor string
//     Date   string
//     Time   string
// }

// Very simple phone validation
func isValidPhone(phone string) bool {
	if len(phone) < 8 || len(phone) > 15 {
		return false
	}
	for _, ch := range phone {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func callExternalAPI[T any](ctx context.Context, url string, target *T) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build API request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s (status %d)", string(bodyBytes), resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	return nil
}

func callExternalAPICallForPost[T any](ctx context.Context, method, url string, body interface{}, target *T) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to build API request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// ✅ Treat 200 and 201 as success
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("failed to parse API response: %w", err)
		}
		return nil
	}

	// ❌ Handle error JSON
	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Status     bool   `json:"status"`
		StatusCode int    `json:"statusCode"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Message != "" {
		// return clean API error message
		return fmt.Errorf(errResp.Message)
	}

	// fallback if JSON not parsable
	return fmt.Errorf("API error: %s (status %d)", string(bodyBytes), resp.StatusCode)
}

func (wc *WhatsAppController) handleNewAppointment(ctx context.Context, userID string, message models.WhatsAppMessage) {
	state, exists := appointmentState[userID]
	if !exists {
		appointmentState[userID] = &AppointmentData{Step: "ask_patient_code_or_phone_number"}
		_ = wc.whatsappService.SendTextMessage(
			userID,
			"🩺 Have you already consulted here before? (Yes/No)",
		)
		return
	}

	switch state.Step {
	case "ask_patient_code_or_phone_number":
		if message.Type == "text" && message.Text != nil {
			ans := strings.ToLower(strings.TrimSpace(message.Text.Body))
			if ans == "yes" {
				state.Step = "await_patient_code_or_phone_number"
				_ = wc.whatsappService.SendTextMessage(userID, "📋 Please enter your patient code:")
			} else if ans == "no" {
				state.Step = "await_patient_name"
				_ = wc.whatsappService.SendTextMessage(userID, "👤 Please enter your full name:")
			} else {
				_ = wc.whatsappService.SendTextMessage(userID, "❌ Please reply Yes or No.")
			}
		}

	case "await_patient_code_or_phone_number":
		code := strings.TrimSpace(message.Text.Body)
		patients, err := wc.verifyPatientCode(code)
		if err != nil || len(patients) == 0 {
			_ = wc.whatsappService.SendTextMessage(userID, "❌ No patient found. Please try again.")
			delete(appointmentState, userID)
			_ = wc.sendMainMenu(userID)
			return
		}

		if len(patients) > 1 {
			state.Step = "choose_patient_from_list"
			_ = wc.sendPatientDetailsList(userID, patients)
			return
		}

		// If exactly one patient found, save directly
		patient := patients[0]
		state.PatientID = patient.ID
		state.PatientCode = patient.PatientCode
		state.PatientName = fmt.Sprintf("%s %s", patient.FirstName, patient.LastName)
		state.PhoneNumber = patient.MobileNumber
		state.Address = patient.Address
		state.DateOfBirth = patient.DateOfBirth
		state.Step = "choose_department"

		_ = wc.sendDepartmentsList(userID)

	case "choose_patient_from_list":
		if message.Interactive != nil && message.Interactive.ListReply != nil {
			selectedID := message.Interactive.ListReply.ID

			// 🔹 Call API again with selected ID/Code
			selectedPatients, err := wc.verifyPatientCode(selectedID)
			if err != nil || len(selectedPatients) == 0 {
				_ = wc.whatsappService.SendTextMessage(userID, "❌ Could not fetch patient details. Please try again.")
				delete(appointmentState, userID)
				_ = wc.sendMainMenu(userID)
				return
			}

			patient := selectedPatients[0]

			// Save only the chosen patient details
			state.PatientID = patient.ID
			state.PatientCode = patient.PatientCode
			state.PatientName = fmt.Sprintf("%s %s", patient.FirstName, patient.LastName)
			state.PhoneNumber = patient.MobileNumber
			state.Address = patient.Address
			state.DateOfBirth = patient.DateOfBirth
			state.Step = "choose_department"

			_ = wc.sendDepartmentsList(userID)
		}

	case "await_patient_name":
		state.PatientName = message.Text.Body
		state.Step = "await_patient_address"
		_ = wc.whatsappService.SendTextMessage(userID, "🏠 Please enter your address:")

	case "await_patient_address":
		state.Address = message.Text.Body
		state.Step = "await_patient_phone"
		_ = wc.whatsappService.SendTextMessage(userID, "📞 Please enter your phone number:")

	case "await_patient_phone":
		state.PhoneNumber = message.Text.Body
		state.Step = "await_patient_dateOfBirth"
		_ = wc.whatsappService.SendTextMessage(userID, "📅 Please enter your date of birth (YYYY-MM-DD):")

	case "await_patient_dateOfBirth":
		state.DateOfBirth = message.Text.Body
		state.Step = "choose_department"
		_ = wc.sendDepartmentsList(userID)

	case "choose_department":
		if message.Type == "interactive" && message.Interactive.ListReply != nil {

			// Convert ID (string) -> uint
			idInt, err := strconv.Atoi(message.Interactive.ListReply.ID)
			if err != nil {
				log.Println("Invalid ID from WhatsApp:", message.Interactive.ListReply.ID, err)
			} else {
				state.DepartmentID = uint(idInt) // ✅ assign to your uint field
			}
			// state.DepartmentID = message.Interactive.ListReply.ID
			state.Step = "await_date"
			_ = wc.whatsappService.SendTextMessage(userID, "📅 Please enter your preferred date (YYYY-MM-DD):")
		}

	case "await_date":
		state.AppointmentDate = message.Text.Body
		state.Step = "choose_doctor"
		_ = wc.sendDoctorsList(userID, state.DepartmentID, state.AppointmentDate)

	case "choose_doctor":
		if message.Type == "interactive" && message.Interactive.ListReply != nil {
			// Convert ID (string) -> uint
			idInt, err := strconv.Atoi(message.Interactive.ListReply.ID)
			if err != nil {
				log.Println("Invalid ID from WhatsApp:", message.Interactive.ListReply.ID, err)
			} else {
				state.DoctorID = uint(idInt) // ✅ assign to your uint field
			}
			// state.DoctorID = message.Interactive.ListReply.ID
			state.DoctorName = message.Interactive.ListReply.Title
			state.Step = "choose_slot"
			_ = wc.sendSlotsList(userID, state.DoctorID, state.AppointmentDate)
		}

	case "choose_slot":
		if message.Type == "interactive" && message.Interactive.ListReply != nil {

			selectedID := message.Interactive.ListReply.ID
			selectedTitle := message.Interactive.ListReply.Title

			// 🔹 Handle pagination
			if selectedID == "more" {
				if slotState[userID] != nil {
					slotState[userID].Page++    // move to next page
					_ = wc.sendSlotPage(userID) // show next batch
				}
				return
			}

			// state.OnlineTempToken = message.Interactive.ListReply.ID

			// Convert ID (string) -> uint
			idInt, err := strconv.Atoi(message.Interactive.ListReply.ID)
			if err != nil {
				log.Println("Invalid ID from WhatsApp:", message.Interactive.ListReply.ID, err)
			} else {
				state.OnlineTempToken = uint(idInt) // ✅ assign to your uint field
			}
			state.TimeSlot = selectedTitle
			state.CreatedFrom = message.From
			success := wc.createAppointment(state, userID)
			if success {
				_ = wc.whatsappService.SendTextMessage(userID,
					fmt.Sprintf("✅ Appointment booked with %s on %s at %s",
						state.DoctorName, state.AppointmentDate, state.TimeSlot))
			} else {
				_ = wc.whatsappService.SendTextMessage(userID, "⚠️ Failed to book appointment. Try again later.")
			}

			log.Println("Appointment state: ", appointmentState)
			b, _ := json.MarshalIndent(state, "", "  ")
			log.Println("Appointment state: ", string(b))

			delete(appointmentState, userID)
			_ = wc.sendMainMenu(userID)
		}
	}
}

// VerifyWebhook handles the webhook verification request from WhatsApp
// func (wc *WhatsAppController) VerifyWebhook(c *gin.Context) {
// 	mode := c.Query("hub.mode")
// 	token := c.Query("hub.verify_token")
// 	challenge := c.Query("hub.challenge")

// 	log.Println("token from whatsApp: ", token)
// 	log.Println("mode from whatsApp: ", mode)
// 	log.Println("challenge from whatsApp: ", challenge)

// 	log.Println("Appointment state: ", appointmentState)
// 	b, _ := json.MarshalIndent(appointmentState, "", "  ")
// 	log.Println("Appointment state: ", string(b))

// 	if mode == "subscribe" && token == wc.whatsappService.GetVerifyToken() {
// 		c.String(http.StatusOK, challenge)
// 		return
// 	}

// 	c.JSON(http.StatusForbidden, gin.H{"error": "Verification failed"})
// }

func (wc *WhatsAppController) VerifyWebhook(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	log.Println("token from WhatsApp:", token)
	log.Println("mode from WhatsApp:", mode)
	log.Println("challenge from WhatsApp:", challenge)

	if mode == "subscribe" {
		if token != wc.whatsappService.GetVerifyToken() {
			log.Println("Verification failed: token mismatch")
			c.JSON(http.StatusForbidden, gin.H{"error": "Token mismatch"})
			return
		}
		c.String(http.StatusOK, challenge)
		return
	}

	log.Println("Verification failed: invalid mode or missing query params")
	c.JSON(http.StatusForbidden, gin.H{"error": "Verification failed"})
}


// func (wc *WhatsAppController) VerifyWebhook(c *gin.Context) {
//     // Debug full request
//     dump, _ := httputil.DumpRequest(c.Request, true)
//     log.Println("Incoming Request:\n", string(dump))

//     mode := c.Query("hub.mode")
//     token := c.Query("hub.verify_token")
//     challenge := c.Query("hub.challenge")

//     log.Println("token from whatsApp: ", token)
//     log.Println("mode from whatsApp: ", mode)
//     log.Println("challenge from whatsApp: ", challenge)

//     if mode == "subscribe" && token == wc.whatsappService.GetVerifyToken() {
//         c.String(http.StatusOK, challenge)
//         return
//     }

//     c.JSON(http.StatusForbidden, gin.H{"error": "Verification failed"})
// }


// HandleWebhook processes incoming WhatsApp messages
func (wc *WhatsAppController) HandleWebhook(c *gin.Context) {
	var webhookData models.WhatsAppWebhookData

	if err := c.ShouldBindJSON(&webhookData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
		log.Println("webhook data binding error", err)
		return
	}


	// Get context for processing
	ctx := c.Request.Context()

	// Process webhook asynchronously to respond quickly
	go wc.processWebhookData(ctx, webhookData)

	// Respond immediately to WhatsApp
	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

// func (wc *WhatsAppController) HandleWebhook(c *gin.Context) {
//     var webhookData models.WhatsAppWebhookData

//     // Log incoming raw body for debugging
//     bodyBytes, err := c.GetRawData()
//     if err != nil {
//         log.Println("[Webhook] Error reading raw body:", err)
//     } else {
//         log.Println("[Webhook] Raw body received:", string(bodyBytes))
//     }

//     // Bind JSON into struct
//     if err := c.ShouldBindJSON(&webhookData); err != nil {
//         log.Println("[Webhook] Error binding JSON:", err)
//         c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
//         return
//     }

//     // Log parsed webhook data
//     b, err := json.MarshalIndent(webhookData, "", "  ")
//     if err != nil {
//         log.Println("[Webhook] Error marshalling webhook data:", err)
//     } else {
//         log.Println("[Webhook] Parsed webhook data:", string(b))
//     }

//     // Log request headers (sometimes useful for debugging)
//     headers, _ := json.MarshalIndent(c.Request.Header, "", "  ")
//     log.Println("[Webhook] Request headers:", string(headers))

//     // Log request metadata
//     log.Println("[Webhook] Remote IP:", c.ClientIP())
//     log.Println("[Webhook] Method:", c.Request.Method)
//     log.Println("[Webhook] URL:", c.Request.URL.String())

//     // Get context for processing
//     ctx := c.Request.Context()

//     // Process webhook asynchronously to respond quickly
//     go func() {
//         log.Println("[Webhook] Processing webhook data asynchronously")
// 		wc.processWebhookData(ctx, webhookData)
// 		log.Println("[Webhook] Webhook data processed successfully")
//     }()

//     // Respond immediately to WhatsApp
//     log.Println("[Webhook] Sending immediate 200 OK response")
//     c.JSON(http.StatusOK, gin.H{"status": "received"})
// }


// processWebhookData processes the webhook data
func (wc *WhatsAppController) processWebhookData(ctx context.Context, webhookData models.WhatsAppWebhookData) {
	log.Println("webhook data", webhookData)
	for _, entry := range webhookData.Entry {
		for _, change := range entry.Changes {
			if change.Field == "messages" {
				wc.processMessages(ctx, change.Value)
			}
		}
	}
}

// processMessages handles incoming messages
func (wc *WhatsAppController) processMessages(ctx context.Context, value models.WhatsAppValue) {
	log.Println("values:", value.Messages, value.Statuses)
	// Process each message
	for _, message := range value.Messages {
		wc.handleIncomingMessage(ctx, message, value.Metadata)
	}

	// Process status updates if needed
	for _, status := range value.Statuses {
		wc.handleStatusUpdate(status)
	}
}

func (wc *WhatsAppController) handleIncomingMessage(ctx context.Context, message models.WhatsAppMessage, metadata models.WhatsAppMetadata) {
	log.Println("Incoming message:", message.Type)
	userID := message.From

	// ========== CASE 0: User says "hi" ==========
	if message.Type == "text" && message.Text != nil {
		userText := strings.TrimSpace(strings.ToLower(message.Text.Body))
		if userText == "hi" || userText == "hello" {
			_ = wc.sendMainMenu(userID)
			return
		}
	}

	// ✅ Before anything else, check if user is in appointment flow
	if _, exists := appointmentState[userID]; exists {
		wc.handleNewAppointment(ctx, userID, message)
		return
	}

	// ========== CASE 1: Awaiting phone number ==========
	if message.Type == "text" && message.Text != nil {
		stateMutex.Lock()
		state := userState[userID]
		stateMutex.Unlock()

		if state == "awaiting_phone" {
			phone := strings.TrimSpace(message.Text.Body)

			if !isValidPhone(phone) {
				_ = wc.whatsappService.SendTextMessage(userID, "❌ Invalid input. Please enter a valid phone number.")
				_ = wc.sendMainMenu(userID)
				delete(userState, userID)
				return
			}

			appointments, err := wc.fetchAppointments(phone)
			if err != nil {
				log.Println("appointment fetching error", err)
				_ = wc.whatsappService.SendTextMessage(userID, "⚠️ Sorry, could not fetch your appointments right now.")
				_ = wc.sendMainMenu(userID)
				return
			}

			if len(appointments) == 0 {
				_ = wc.whatsappService.SendTextMessage(userID, "❌ You do not have any active appointments.")
				_ = wc.sendMainMenu(userID)
				return
			}

			_ = wc.sendAppointmentsList(userID, appointments)

			stateMutex.Lock()
			delete(userState, userID)
			stateMutex.Unlock()
			return
		}
	}

	// ========== CASE 2: Interactive messages ==========
	switch message.Type {
	case "interactive":
		if message.Interactive != nil {
			if message.Interactive.ButtonReply != nil {
				switch message.Interactive.ButtonReply.ID {
				case "my_appointment":
					_ = wc.whatsappService.SendTextMessage(userID, "📞 Please enter your phone number to view appointments:")
					stateMutex.Lock()
					userState[userID] = "awaiting_phone"
					stateMutex.Unlock()
					return

				case "new_appointment":
					wc.handleNewAppointment(ctx, userID, message)
					return

				case "contact_us":
					_ = wc.whatsappService.SendTextMessage(userID, "📞 Contact us at: +91-98765-43210")
					_ = wc.sendMainMenu(userID)
					return
				}
			}

			if message.Interactive.ListReply != nil {
				apptID := message.Interactive.ListReply.ID
				log.Println("Appointment id: ", apptID)
				details, err := wc.getAppointmentDetails(apptID)

				if err != nil {
					log.Println("Error fetching appointment details:", err)
					_ = wc.whatsappService.SendTextMessage(userID, "⚠️ Failed to fetch appointment details. Try again later.")
					_ = wc.sendMainMenu(userID)
					return
				}

				if details != "" {
					_ = wc.whatsappService.SendTextMessage(userID, details)
				} else {
					_ = wc.whatsappService.SendTextMessage(userID, "❓ Appointment not found.")
				}

				_ = wc.sendMainMenu(userID)
				return
			}

			// if message.Interactive.ListReply != nil {
			// 	// You still extract the ID if you need it for logging, auditing, etc.
			// 	// apptID := message.Interactive.ListReply.ID
			// 	// _ = wc.getAppointmentDetails(apptID) // optional — remove if not needed

			// 	// Just send the main menu
			// 	_ = wc.sendMainMenu(userID)

			// 	return
			// }

		}
	}

	// ========== CASE 3: Default fallback ==========
	_ = wc.whatsappService.SendTextMessage(userID, "🤔 Sorry, I didn’t understand that.")
	_ = wc.sendMainMenu(userID)
}

// ========================
// Appointment API Call
// ========================
type Appointment struct {
	ID          int    `json:"appointmentId"`
	Doctor      int    `json:"doctorId"` // You can later map this to doctor name if needed
	DoctorName  string `json:"doctorName"`
	TokenNumber int    `json:"tokenNumber"`
	Date        string `json:"date"`
	Time        string `json:"time"`
}

// Raw API response struct
type apiResponse struct {
	Status     bool   `json:"status"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       []struct {
		AppointmentID       int    `json:"appointmentId"`
		DoctorID            int    `json:"doctorId"`
		DoctorName          string `json:"doctorName"`
		AppointmentDateTime string `json:"appointmentDateTime"`
		TimeSlot            string `json:"timeSlot"`
		TokenNumber         int    `json:"tokenNumber"`
	} `json:"data"`
}

type Patient struct {
	ID           int    `json:"patientId"`
	PatientCode  string `json:"patientCode"` // You can later map this to doctor name if needed
	Salutation   string `json:"salutation"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	DateOfBirth  string `json:"dateOfBirth"`
	MobileNumber string `json:"mobileNumber"`
	Address      string `json:"address"`
}

type apiPatientResponse struct {
	Status     bool   `json:"status"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       []struct {
		PatientID    int    `json:"patientId"`
		PatientCode  string `json:"patientCode"`
		Salutation   string `json:"salutation"`
		FirstName    string `json:"firstName"`
		LastName     string `json:"lastName"`
		DateOfBirth  string `json:"dateOfBirth"`
		MobileNumber string `json:"mobileNumber"`
		Address      string `json:"address"`
	} `json:"data"`
}

type Department struct {
	ID             int    `json:"departmentId"`
	DepartmentName string `json:"departmentName"`
}

type apiDepartmentResponse struct {
	Status     bool   `json:"status"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       []struct {
		DepartmentID   int    `json:"departmentId"`
		DepartmentName string `json:"departmentName"`
	} `json:"data"`
}

type Doctor struct {
	ID         int    `json:"employeeId"`
	DoctorName string `json:"doctorName"`
}

type apiDoctorResponse struct {
	Status     bool   `json:"status"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       []struct {
		EmployeeID int    `json:"employeeId"`
		FirstName  string `json:"firstName"`
		LastName   string `json:"lastName"`
		IsOnLeave  bool   `json:"isOnLeave"`
	} `json:"data"`
}

type BookedSlotDTO struct {
	TimeSlot string `json:"timeSlot"`
}

type Availability struct {
	ID          int             `json:"availabilityId"`
	DayOfWeek   string          `json:"dayOfWeek"`
	WeekType    string          `json:"weekType"`
	Start       string          `json:"availableTimeStart"`
	End         string          `json:"availableTimeEnd"`
	BookedSlots []BookedSlotDTO `json:"bookedSlots"`
}

type apiDoctorAvailabilityResponse struct {
	Status     bool   `json:"status"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       []struct {
		DayOfWeek          string          `json:"dayOfWeek"`
		WeekType           string          `json:"weekType"`
		AvailabilityID     int             `json:"availabilityId"`
		AvailableTimeStart string          `json:"availableTimeStart"`
		AvailableTimeEnd   string          `json:"availableTimeEnd"`
		BookedSlots        []BookedSlotDTO `json:"bookedSlots"`
	} `json:"data"`
}

type apiAppointmentResponse struct {
	Status     bool   `json:"status"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Data       []struct {
		TempAppointmentID int `json:"tempAppointmentId"`
	} `json:"data"`
}

type SlotState struct {
	Slots    []string
	Page     int
	PageSize int
}

var slotState = make(map[string]*SlotState) // userID → state

func truncate(str string, max int) string {
	if len(str) <= max {
		return str
	}
	return str[:max-1] + "…" // add ellipsis
}

func generateTimeSlots(start, end string) ([]string, error) {
	layout := "15:04:05"
	tStart, err := time.Parse(layout, start)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}
	tEnd, err := time.Parse(layout, end)
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}

	slots := []string{}
	for t := tStart; t.Before(tEnd); t = t.Add(15 * time.Minute) {
		slots = append(slots, t.Format("15:04"))
	}
	return slots, nil
}

func chunkSlotsIntoSections(slots []string, title string) []models.Section {
	sections := []models.Section{}
	for i := 0; i < len(slots); i += 10 { // WhatsApp allows max 10 rows per section
		end := i + 10
		if end > len(slots) {
			end = len(slots)
		}

		rows := []models.ListItem{}
		for j, slot := range slots[i:end] {
			rows = append(rows, models.ListItem{
				ID:    strconv.Itoa(i + j + 1), // sequential IDs 1,2,3,...
				Title: slot,                    // e.g. "09:00"
			})
		}

		sectionTitle := fmt.Sprintf("%s (Part %d)", title, (i/10)+1)
		sections = append(sections, models.Section{Title: sectionTitle, Rows: rows})
	}
	return sections
}

func (wc *WhatsAppController) fetchAppointments(phone string) ([]Appointment, error) {

	// 🔹 Force a fresh background context, safe timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://61.2.142.81:8082/api/appointment/search?phoneNumber=%s", phone)

	var apiResp apiResponse
	if err := callExternalAPI(ctx, url, &apiResp); err != nil {
		log.Println("API fetching error", err)
		return nil, err
	}

	// Convert to your model
	appointments := make([]Appointment, len(apiResp.Data))
	for i, d := range apiResp.Data {
		t, err := time.Parse("2006-01-02T15:04:05", d.AppointmentDateTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse appointmentDateTime: %w", err)
		}

		var timeStr string
		if d.TimeSlot != "" {
			timeStr = d.TimeSlot
		} else {
			timeStr = t.Format("03:04 PM")
		}

		appointments[i] = Appointment{
			ID:         d.AppointmentID,
			Doctor:     d.DoctorID,
			DoctorName: d.DoctorName,
			Date:       t.Format("2006-01-02"),
			Time:       timeStr,
		}
	}

	b, _ := json.MarshalIndent(appointments, "", "  ")
	log.Println("Appointments", string(b))

	return appointments, nil
}

// ========================
// Send List of Appointments
// ========================
func (wc *WhatsAppController) sendAppointmentsList(to string, appointments []Appointment) error {

	if len(appointments) == 0 {
		return wc.whatsappService.SendTextMessage(to, "⚠ No appointments found.")
	}

	// 🔹 If only one appointment, just send text message
	if len(appointments) == 1 {
		appt := appointments[0]
		msg := fmt.Sprintf(
			"📅 Appointment Details\n\nDoctor: Dr. %s\nDate: %s\nTime: %s",
			appt.DoctorName, appt.Date, appt.Time,
		)
		return wc.whatsappService.SendTextMessage(to, msg)
	}

	rows := make([]models.ListItem, 0, len(appointments))

	for _, appt := range appointments {
		rows = append(rows, models.ListItem{
			ID:          strconv.Itoa(appt.ID),
			Title:       fmt.Sprintf("Dr. %s", appt.DoctorName),
			Description: fmt.Sprintf("Date: %s, Time: %s", appt.Date, appt.Time),
		})
	}

	sections := []models.Section{
		{
			Title: "Your Appointments",
			Rows:  rows,
		},
	}

	interactive := &models.InteractiveMessage{
		Type: "list",
		Header: &models.MessageHeader{
			Type: "text",
			Text: "📅 My Appointments",
		},
		Body: &models.InteractiveBody{
			Text: "Select one appointment for details:",
		},
		Footer: &models.InteractiveFooter{
			Text: "Clinic Support",
		},
		Action: &models.InteractiveAction{
			Button:   "Choose Appointment",
			Sections: sections,
		},
	}

	return wc.whatsappService.SendInteractiveMessage(to, interactive)
}

// ========================
// Appointment Details
// ========================
func (wc *WhatsAppController) getAppointmentDetails(apptID string) (string, error) {
	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://61.2.142.81:8082/api/appointment/get-by-id?appointmentId=%s", apptID)

	var apiResp apiResponse
	if err := callExternalAPI(ctx, url, &apiResp); err != nil {
		log.Println("API fetching error", err)
		return "", err
	}

	if len(apiResp.Data) == 0 {
		return "❓ Appointment not found.", nil
	}

	d := apiResp.Data[0] // since it's a single appointment by ID

	t, err := time.Parse("2006-01-02T15:04:05", d.AppointmentDateTime)
	if err != nil {
		return "", fmt.Errorf("failed to parse appointmentDateTime: %w", err)
	}

	var timeStr string
	if d.TimeSlot != "" {
		timeStr = d.TimeSlot
	} else {
		timeStr = t.Format("03:04 PM")
	}

	// ✅ Format a WhatsApp-friendly message
	msg := fmt.Sprintf(
		"✅ Appointment Details\n👨‍⚕️ Doctor: %s\n📅 Date: %s\n⏰ Time: %s\n🔢 Token: %d",
		d.DoctorName,
		t.Format("2006-01-02"),
		timeStr,
		d.TokenNumber,
	)

	return msg, nil
}

// ==============================================
// Fetching Patient Details By Code Or Phone number
// ===============================================

func (wc *WhatsAppController) verifyPatientCode(code string) ([]Patient, error) {

	// 🔹 Force a fresh background context, safe timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://61.2.142.81:8082/api/patient/search?userInput=%s", code)

	var apiResp apiPatientResponse
	if err := callExternalAPI(ctx, url, &apiResp); err != nil {
		log.Println("API fetching error", err)
		return nil, err
	}

	// Convert to your model
	patients := make([]Patient, len(apiResp.Data))
	for i, d := range apiResp.Data {

		patients[i] = Patient{
			ID:           d.PatientID,
			PatientCode:  d.PatientCode,
			Salutation:   d.Salutation,
			FirstName:    d.FirstName,
			LastName:     d.LastName,
			DateOfBirth:  d.DateOfBirth,
			MobileNumber: d.MobileNumber,
			Address:      d.Address,
		}
	}

	b, _ := json.MarshalIndent(patients, "", "  ")
	log.Println("Patients", string(b))

	return patients, nil
	// if code == "P123" {
	// 	return true, AppointmentData{
	// 		Name:    "John Doe",
	// 		Phone:   "+919876543210",
	// 		Address: "123 Main Street",
	// 	}
	// }
	// return false, AppointmentData{}
}

func (wc *WhatsAppController) sendPatientDetailsList(to string, patients []Patient) error {
	rows := make([]models.ListItem, 0, len(patients))

	for _, appt := range patients {
		fullName := fmt.Sprintf("%s %s", appt.FirstName, appt.LastName)

		rows = append(rows, models.ListItem{
			ID:          strconv.Itoa(appt.ID),
			Title:       truncate(fullName, 24), // short for list
			Description: truncate(fmt.Sprintf("Patient Code: %s | %s", appt.PatientCode, fullName), 72),
		})
	}

	sections := []models.Section{
		{
			Title: "Search Results",
			Rows:  rows,
		},
	}

	interactive := &models.InteractiveMessage{
		Type: "list",
		Header: &models.MessageHeader{
			Type: "text",
			Text: "📅 Patient Details",
		},
		Body: &models.InteractiveBody{
			Text: "Choose one patient for appointment:",
		},
		Footer: &models.InteractiveFooter{
			Text: "Clinic Support",
		},
		Action: &models.InteractiveAction{
			Button:   "Choose Patient",
			Sections: sections,
		},
	}

	return wc.whatsappService.SendInteractiveMessage(to, interactive)
}

func (wc *WhatsAppController) sendDepartmentsList(userID string) error {

	// 🔹 Force a fresh background context, safe timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := "http://61.2.142.81:8082/api/department/list"

	var apiResp apiDepartmentResponse
	if err := callExternalAPI(ctx, url, &apiResp); err != nil {
		log.Println("API fetching error", err)
		return err
	}

	// Convert to your model
	departments := make([]Department, len(apiResp.Data))
	for i, d := range apiResp.Data {

		departments[i] = Department{
			ID:             d.DepartmentID,
			DepartmentName: d.DepartmentName,
		}
	}

	b, _ := json.MarshalIndent(departments, "", "  ")
	log.Println("departments", string(b))

	rows := make([]models.ListItem, 0, len(departments))

	for _, dept := range departments {

		rows = append(rows, models.ListItem{
			ID:    strconv.Itoa(dept.ID),
			Title: dept.DepartmentName,
		})
	}

	interactive := &models.InteractiveMessage{
		Type: "list",
		Body: &models.InteractiveBody{Text: "Please select a department"},
		Action: &models.InteractiveAction{
			Button: "Choose",
			Sections: []models.Section{
				{Title: "Departments", Rows: rows},
			},
		},
	}
	return wc.whatsappService.SendInteractiveMessage(userID, interactive)
}

func (wc *WhatsAppController) sendDoctorsList(userID string, dept uint, date string) error {
	// 🔹 Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// deptInt, err := strconv.Atoi(dept)
	// if err != nil {
	// 	return fmt.Errorf("invalid departmentId: %v", err)
	// }

	url := fmt.Sprintf(
		"http://61.2.142.81:8082/api/doctor/list?employeeType=%d&departmentId=%d&inputDate=%s",
		1, dept, date,
	)

	var apiResp apiDoctorResponse
	if err := callExternalAPI(ctx, url, &apiResp); err != nil {
		log.Println("API fetching error", err)
		return err
	}

	// Convert to your model
	// doctors := make([]Doctor, len(apiResp.Data))
	// for i, d := range apiResp.Data {
	// 	doctors[i] = Doctor{
	// 		ID:         d.EmployeeID,
	// 		DoctorName: fmt.Sprintf("%s %s", d.FirstName, d.LastName),
	// 	}
	// }

	// Convert to your model (only doctors not on leave)
	doctors := make([]Doctor, 0)
	for _, d := range apiResp.Data {
		if !d.IsOnLeave {
			doctors = append(doctors, Doctor{
				ID:         d.EmployeeID,
				DoctorName: fmt.Sprintf("Dr.%s %s", d.FirstName, d.LastName),
			})
		}
	}

	b, _ := json.MarshalIndent(doctors, "", "  ")
	log.Println("doctors", string(b))

	if len(doctors) == 0 {
		_ = wc.whatsappService.SendTextMessage(userID, "❌ No doctors available in this department on the selected date.")
		delete(appointmentState, userID)
		_ = wc.sendMainMenu(userID)
		return nil
	}

	// Build WhatsApp list items
	rows := make([]models.ListItem, 0, len(doctors))
	for _, doctor := range doctors {
		rows = append(rows, models.ListItem{
			ID:    strconv.Itoa(doctor.ID), // unique identifier for callback
			Title: doctor.DoctorName,       // 👨‍⚕️ main label
			// Description: "Some extra info",  // optional: specialization, timings etc.
		})
	}

	interactive := &models.InteractiveMessage{
		Type: "list",
		Header: &models.MessageHeader{
			Type: "text",
			Text: "👨‍⚕️ Available Doctors",
		},
		Body: &models.InteractiveBody{
			Text: "Please select a doctor from the list below:",
		},
		Footer: &models.InteractiveFooter{
			Text: "Clinic Support",
		},
		Action: &models.InteractiveAction{
			Button: "Choose Doctor",
			Sections: []models.Section{
				{Title: "Doctors", Rows: rows},
			},
		},
	}

	return wc.whatsappService.SendInteractiveMessage(userID, interactive)
}

func (wc *WhatsAppController) sendSlotsList(userID string, doctor uint, date string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf(
		"http://61.2.142.81:8082/api/doctorAvailability/byDate?doctorId=%d&inputDate=%s",
		doctor, date,
	)

	var apiResp apiDoctorAvailabilityResponse
	if err := callExternalAPI(ctx, url, &apiResp); err != nil {
		log.Println("API fetching error", err)
		return err
	}

	// Collect all FREE slots
	allSlots := []string{}
	for _, avail := range apiResp.Data {
		slots, err := generateTimeSlots(avail.AvailableTimeStart, avail.AvailableTimeEnd)
		if err != nil {
			log.Println("Error generating slots:", err)
			continue
		}

		booked := make(map[string]bool)
		for _, b := range avail.BookedSlots {
			booked[b.TimeSlot] = true
		}

		for _, s := range slots {
			t, err := time.Parse("15:04", s)
			if err != nil {
				continue
			}
			formatted := t.Format("03:04 PM")
			if !booked[formatted] {
				allSlots = append(allSlots, formatted)
			}
		}
	}

	if len(allSlots) == 0 {
		_ = wc.whatsappService.SendTextMessage(userID, "❌ No available slots found for this doctor.")
		delete(slotState, userID)
		return nil
	}

	// Save state for user
	slotState[userID] = &SlotState{
		Slots:    allSlots,
		Page:     0,
		PageSize: 10,
	}

	// Send first page
	return wc.sendSlotPage(userID)
}

func (wc *WhatsAppController) sendSlotPage(userID string) error {
	state, ok := slotState[userID]
	if !ok {
		return wc.whatsappService.SendTextMessage(userID, "⚠ No slots available.")
	}

	// Always reserve space for "Next Slots"
	pageSize := 9

	start := state.Page * pageSize
	if start >= len(state.Slots) {
		_ = wc.whatsappService.SendTextMessage(userID, "✅ No more slots.")
		delete(slotState, userID)
		return nil
	}

	end := start + pageSize
	if end > len(state.Slots) {
		end = len(state.Slots)
	}

	rows := []models.ListItem{}
	for i, slot := range state.Slots[start:end] {
		rows = append(rows, models.ListItem{
			ID:    strconv.Itoa(start + i + 1),
			Title: slot,
		})
	}

	// Add "Next Slots" row only if more remain
	if end < len(state.Slots) {
		rows = append(rows, models.ListItem{
			ID:    "more",
			Title: "➡ Next Slots",
		})
	}

	section := models.Section{
		Title: fmt.Sprintf("Slots %d - %d", start+1, end),
		Rows:  rows,
	}

	totalPages := (len(state.Slots) + pageSize - 1) / pageSize

	interactive := &models.InteractiveMessage{
		Type: "list",
		Header: &models.MessageHeader{
			Type: "text",
			Text: "⏰ Available Time Slots",
		},
		Body: &models.InteractiveBody{
			Text: fmt.Sprintf("Page %d of %d", state.Page+1, totalPages),
		},
		Action: &models.InteractiveAction{
			Button:   "Choose Slot",
			Sections: []models.Section{section},
		},
	}

	return wc.whatsappService.SendInteractiveMessage(userID, interactive)
}


func (wc *WhatsAppController) createAppointment(data *AppointmentData, userID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := "http://61.2.142.81:8082/api/tempAppointment/create"

	var resp apiAppointmentResponse

	err := callExternalAPICallForPost(ctx, http.MethodPost, url, data, &resp)
	if err != nil {
		// ❌ Send the exact error back to user
		_ = wc.whatsappService.SendTextMessage(userID,
			fmt.Sprintf("⚠️ Appointment could not be created: %s", err.Error()))

		log.Println("❌ Appointment API error:", err)
		// delete(appointmentState, userID)
		// _ = wc.sendMainMenu(userID)
		return false
	}

	if !resp.Status {
		// ❌ Handle logical failure from API (even if status 200/201)
		_ = wc.whatsappService.SendTextMessage(userID,
			fmt.Sprintf("⚠️ Appointment failed: %s", resp.Message))

		log.Printf("❌ Appointment creation failed: %s", resp.Message)
		delete(appointmentState, userID)
		_ = wc.sendMainMenu(userID)
		return false
	}

	log.Printf("✅ Appointment created successfully: %+v", resp)
	_ = wc.whatsappService.SendTextMessage(userID,
		"✅ Appointment created successfully!")
	return true
}

// ========================
// Main Menu Buttons
// ========================
func (wc *WhatsAppController) sendMainMenu(to string) error {
	interactive := &models.InteractiveMessage{
		Type: "button",
		Body: &models.InteractiveBody{
			Text: "👋 Hi! How can we help you today?",
		},
		Footer: &models.InteractiveFooter{
			Text: "Clinic Support",
		},
		Action: &models.InteractiveAction{
			Buttons: []models.InteractiveButton{
				{
					Type: "reply",
					Reply: &models.ButtonReply{
						ID:    "my_appointment",
						Title: "📅 My Appointment",
					},
				},
				{
					Type: "reply",
					Reply: &models.ButtonReply{
						ID:    "new_appointment",
						Title: "🆕 New Appointment",
					},
				},
				{
					Type: "reply",
					Reply: &models.ButtonReply{
						ID:    "contact_us",
						Title: "📞 Contact Us",
					},
				},
			},
		},
	}
	return wc.whatsappService.SendInteractiveMessage(to, interactive)
}

// handleStatusUpdate processes message status updates
func (wc *WhatsAppController) handleStatusUpdate(status models.WhatsAppStatus) {
	// Log status updates or update message status in database
	fmt.Printf("Message %s to %s: %s\n", status.ID, status.RecipientID, status.Status)

	// Handle any errors
	if len(status.Errors) > 0 {
		for _, err := range status.Errors {
			fmt.Printf("WhatsApp Error: %d - %s: %s\n", err.Code, err.Title, err.Message)
		}
	}
}

// SendMessage sends a message to a specific WhatsApp number (for notifications)
func (wc *WhatsAppController) SendMessage(c *gin.Context) {
	var req struct {
		To      string `json:"to" binding:"required"`
		Message string `json:"message" binding:"required"`
		Type    string `json:"type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Clean phone number
	to := wc.whatsappService.CleanPhoneNumber(req.To)

	var err error
	switch req.Type {
	case "template":
		// Handle template messages if needed
		err = wc.whatsappService.SendTextMessage(to, req.Message)
	default:
		err = wc.whatsappService.SendTextMessage(to, req.Message)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to send message",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "sent",
		"to":     to,
	})
}

// GetStatus returns WhatsApp service status
func (wc *WhatsAppController) GetStatus(c *gin.Context) {
	status := wc.whatsappService.GetStatus()
	c.JSON(http.StatusOK, status)
}
