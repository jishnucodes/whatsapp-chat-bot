// controllers/whatsapp_controller.go
package controllers

import (
	"context"
	"encoding/json"
	// "errors"
	"io"
	"strconv"
	"sync"
	"time"

	// "encoding/json"
	"fmt"
	"log"
	"net/http"
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
	PatientCode string
	Name        string
	Address     string
	Phone       string
	Department  string
	Date        string
	Doctor      string
	Slot        string
	Step        string
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

// Pretend API check for patient code
func (wc *WhatsAppController) verifyPatientCode(code string) (bool, AppointmentData) {
	if code == "P123" {
		return true, AppointmentData{
			Name:    "John Doe",
			Phone:   "+919876543210",
			Address: "123 Main Street",
		}
	}
	return false, AppointmentData{}
}

// Mock departments
func (wc *WhatsAppController) sendDepartmentsList(userID string) error {
	depts := []models.ListItem{
		{ID: "cardiology", Title: "❤️ Cardiology"},
		{ID: "dermatology", Title: "🩺 Dermatology"},
		{ID: "pediatrics", Title: "👶 Pediatrics"},
	}
	interactive := &models.InteractiveMessage{
		Type: "list",
		Body: &models.InteractiveBody{Text: "Please select a department"},
		Action: &models.InteractiveAction{
			Button: "Choose",
			Sections: []models.Section{
				{Title: "Departments", Rows: depts},
			},
		},
	}
	return wc.whatsappService.SendInteractiveMessage(userID, interactive)
}

// Mock doctors
func (wc *WhatsAppController) sendDoctorsList(userID, dept, date string) error {
	doctors := []models.ListItem{
		{ID: "dr_smith", Title: "Dr. Smith"},
		{ID: "dr_jones", Title: "Dr. Jones"},
	}
	interactive := &models.InteractiveMessage{
		Type: "list",
		Body: &models.InteractiveBody{Text: "Select a doctor"},
		Action: &models.InteractiveAction{
			Button: "Choose",
			Sections: []models.Section{
				{Title: "Doctors", Rows: doctors},
			},
		},
	}
	return wc.whatsappService.SendInteractiveMessage(userID, interactive)
}

// Mock slots
func (wc *WhatsAppController) sendSlotsList(userID, doctor, date string) error {
	slots := []models.ListItem{
		{ID: "10am", Title: "10:00 AM"},
		{ID: "11am", Title: "11:00 AM"},
		{ID: "2pm", Title: "02:00 PM"},
	}
	interactive := &models.InteractiveMessage{
		Type: "list",
		Body: &models.InteractiveBody{Text: "Select a slot"},
		Action: &models.InteractiveAction{
			Button: "Choose",
			Sections: []models.Section{
				{Title: "Available Slots", Rows: slots},
			},
		},
	}
	return wc.whatsappService.SendInteractiveMessage(userID, interactive)
}

// Mock appointment creation
func (wc *WhatsAppController) createAppointment(data *AppointmentData) bool {
	log.Printf("📅 Creating appointment: %+v", data)
	return true // always success for mock
}

func (wc *WhatsAppController) handleNewAppointment(ctx context.Context, userID string, message models.WhatsAppMessage) {
	state, exists := appointmentState[userID]
	if !exists {
		appointmentState[userID] = &AppointmentData{Step: "ask_patient_code"}
		_ = wc.whatsappService.SendTextMessage(userID, "🆕 Do you have a patient code? (Yes/No)")
		return
	}

	switch state.Step {
	case "ask_patient_code":
		if message.Type == "text" && message.Text != nil {
			ans := strings.ToLower(strings.TrimSpace(message.Text.Body))
			if ans == "yes" {
				state.Step = "await_patient_code"
				_ = wc.whatsappService.SendTextMessage(userID, "📋 Please enter your patient code:")
			} else if ans == "no" {
				state.Step = "await_patient_name"
				_ = wc.whatsappService.SendTextMessage(userID, "👤 Please enter your full name:")
			} else {
				_ = wc.whatsappService.SendTextMessage(userID, "❌ Please reply Yes or No.")
			}
		}

	case "await_patient_code":
		state.PatientCode = message.Text.Body
		valid, details := wc.verifyPatientCode(state.PatientCode)
		if !valid {
			_ = wc.whatsappService.SendTextMessage(userID, "❌ Invalid patient code. Please try again.")
			return
		}
		state.Name = details.Name
		state.Phone = details.Phone
		state.Address = details.Address
		state.Step = "choose_department"
		_ = wc.sendDepartmentsList(userID)

	case "await_patient_name":
		state.Name = message.Text.Body
		state.Step = "await_patient_address"
		_ = wc.whatsappService.SendTextMessage(userID, "🏠 Please enter your address:")

	case "await_patient_address":
		state.Address = message.Text.Body
		state.Step = "await_patient_phone"
		_ = wc.whatsappService.SendTextMessage(userID, "📞 Please enter your phone number:")

	case "await_patient_phone":
		state.Phone = message.Text.Body
		state.Step = "choose_department"
		_ = wc.sendDepartmentsList(userID)

	case "choose_department":
		if message.Type == "interactive" && message.Interactive.ListReply != nil {
			state.Department = message.Interactive.ListReply.ID
			state.Step = "await_date"
			_ = wc.whatsappService.SendTextMessage(userID, "📅 Please enter your preferred date (YYYY-MM-DD):")
		}

	case "await_date":
		state.Date = message.Text.Body
		state.Step = "choose_doctor"
		_ = wc.sendDoctorsList(userID, state.Department, state.Date)

	case "choose_doctor":
		if message.Type == "interactive" && message.Interactive.ListReply != nil {
			state.Doctor = message.Interactive.ListReply.ID
			state.Step = "choose_slot"
			_ = wc.sendSlotsList(userID, state.Doctor, state.Date)
		}

	case "choose_slot":
		if message.Type == "interactive" && message.Interactive.ListReply != nil {
			state.Slot = message.Interactive.ListReply.ID
			success := wc.createAppointment(state)
			if success {
				_ = wc.whatsappService.SendTextMessage(userID,
					fmt.Sprintf("✅ Appointment booked with Dr. %s on %s at %s",
						state.Doctor, state.Date, state.Slot))
			} else {
				_ = wc.whatsappService.SendTextMessage(userID, "⚠️ Failed to book appointment. Try again later.")
			}

			delete(appointmentState, userID)
			_ = wc.sendMainMenu(userID)
		}
	}
}

// VerifyWebhook handles the webhook verification request from WhatsApp
func (wc *WhatsAppController) VerifyWebhook(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	log.Println("token from whatsApp: ", token)
	log.Println("mode from whatsApp: ", mode)
	log.Println("challenge from whatsApp: ", challenge)

	if mode == "subscribe" && token == wc.whatsappService.GetVerifyToken() {
		c.String(http.StatusOK, challenge)
		return
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "Verification failed"})
}

// HandleWebhook processes incoming WhatsApp messages
func (wc *WhatsAppController) HandleWebhook(c *gin.Context) {
	var webhookData models.WhatsAppWebhookData

	if err := c.ShouldBindJSON(&webhookData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
		return
	}

	// Get context for processing
	ctx := c.Request.Context()

	// Process webhook asynchronously to respond quickly
	go wc.processWebhookData(ctx, webhookData)

	// Respond immediately to WhatsApp
	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

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
	ID     int    `json:"appointmentId"`
	Doctor int    `json:"doctorId"` // You can later map this to doctor name if needed
    DoctorName string `json:"doctorName"`
    TokenNumber  int  `json:"tokenNumber"`
	Date   string `json:"date"`
	Time   string `json:"time"`
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

func (wc *WhatsAppController) fetchAppointments(phone string) ([]Appointment, error) {

	// 🔹 Force a fresh background context, safe timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://61.2.142.81:8086/api/appointment/search?phoneNumber=%s", phone)

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

		appointments[i] = Appointment{
			ID:     d.AppointmentID,
			Doctor: d.DoctorID,
            DoctorName: d.DoctorName,
			Date:   t.Format("2006-01-02"),
			Time:   d.TimeSlot,
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

    url := fmt.Sprintf("http://61.2.142.81:8086/api/appointment/get-by-id?appointmentId=%s", apptID)

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

    // ✅ Format a WhatsApp-friendly message
    msg := fmt.Sprintf(
        "✅ Appointment Details\n👨‍⚕️ Doctor: %s\n📅 Date: %s\n⏰ Time: %s\n🔢 Token: %d",
        d.DoctorName,
        t.Format("2006-01-02"),
        d.TimeSlot,
        d.TokenNumber,
    )

    return msg, nil
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
