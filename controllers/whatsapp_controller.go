// controllers/whatsapp_controller.go
package controllers

import (
	"context"
	"encoding/json"
	"io"
	"sync"

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

// Track user states
var (
    userState   = make(map[string]string) // userID -> state
    stateMutex  sync.Mutex
)

// Appointment structure
type Appointment struct {
    ID     string
    Doctor string
    Date   string
    Time   string
}

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

func (wc *WhatsAppController) callExternalAPI(url string) string {
    req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
    if err != nil {
        log.Printf("Failed to build API request: %v", err)
        return "⚠️ Something went wrong. Please try again."
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("API request failed: %v", err)
        return "⚠️ Could not reach our service right now."
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("Failed to read API response: %v", err)
        return "⚠️ Error reading response."
    }

    // Unmarshal into slice of posts
    var posts []Post
    if err := json.Unmarshal(body, &posts); err != nil {
        log.Printf("Failed to parse API response: %v", err)
        return "⚠️ Error parsing response."
    }

    if len(posts) > 0 {
        return posts[0].Body // ✅ just return the body field
    }

    return "⚠️ No data found."
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


	// ========== CASE 1: Awaiting phone number ==========
    if message.Type == "text" && message.Text != nil {
        stateMutex.Lock()
        state := userState[userID]
        stateMutex.Unlock()

        if state == "awaiting_phone" {
            phone := strings.TrimSpace(message.Text.Body)

			// ✅ Simple validation for phone number
            if !isValidPhone(phone) {
                _ = wc.whatsappService.SendTextMessage(userID, "❌ Invalid input. Please enter a valid phone number.")
                _ = wc.sendMainMenu(userID)
                delete(userState, userID) // reset state
                return
            }

            appointments, err := wc.fetchAppointments(ctx, phone)
            if err != nil {
                _ = wc.whatsappService.SendTextMessage(userID, "⚠️ Sorry, could not fetch your appointments right now.")
                _ = wc.sendMainMenu(userID)
                return
            }

            if len(appointments) == 0 {
                _ = wc.whatsappService.SendTextMessage(userID, "❌ You do not have any active appointments.")
                _ = wc.sendMainMenu(userID)
                return
            }

            // ✅ Send as list menu
            _ = wc.sendAppointmentsList(userID, appointments)

            // Clear state
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
            // Handle Button Reply
            if message.Interactive.ButtonReply != nil {
                switch message.Interactive.ButtonReply.ID {
                case "my_appointment":
                    _ = wc.whatsappService.SendTextMessage(userID, "📞 Please enter your phone number to view appointments:")
                    stateMutex.Lock()
                    userState[userID] = "awaiting_phone"
                    stateMutex.Unlock()
                    return

                case "new_appointment":
                    _ = wc.whatsappService.SendTextMessage(userID, "🆕 Please visit our booking portal: https://clinic-booking.com/new")
                    _ = wc.sendMainMenu(userID)
                    return

                case "contact_us":
                    _ = wc.whatsappService.SendTextMessage(userID, "📞 Contact us at: +91-98765-43210")
                    _ = wc.sendMainMenu(userID)
                    return
                }
            }

            // Handle List Reply
            if message.Interactive.ListReply != nil {
                apptID := message.Interactive.ListReply.ID
                details := wc.getAppointmentDetails(apptID)

                if details != "" {
                    _ = wc.whatsappService.SendTextMessage(userID, details)
                } else {
                    _ = wc.whatsappService.SendTextMessage(userID, "❓ Appointment not found.")
                }

                // Back to main menu
				 _ = wc.whatsappService.SendTextMessage(userID, "🤔 Sorry, I didn’t understand that.")
                _ = wc.sendMainMenu(userID)
                return
            }
        }
    }

    // ========== Default: Send main menu ==========
    _ = wc.sendMainMenu(userID)

	// Check if it's a text message
	// if message.Type == "text" && message.Text != nil {
	// 	userText := strings.TrimSpace(strings.ToLower(message.Text.Body))
	// 	if userText == "hi" || userText == "hello" {
	// 		// Send interactive buttons instead of plain text
	// 		interactive := &models.InteractiveMessage{
	// 			Type: "button",
	// 			Body: &models.InteractiveBody{
	// 				Text: "Hi! How can we help you today?",
	// 			},
	// 			Footer: &models.InteractiveFooter{
	// 				Text: "Sanitas Landscaping",
	// 			},
	// 			Action: &models.InteractiveAction{
	// 				Buttons: []models.InteractiveButton{
	// 					{
	// 						Type: "reply",
	// 						Reply: &models.ButtonReply{
	// 							ID:    "help_plant_care",
	// 							Title: "Plant Care",
	// 						},
	// 					},
	// 					{
	// 						Type: "reply",
	// 						Reply: &models.ButtonReply{
	// 							ID:    "help_landscaping",
	// 							Title: "Landscaping",
	// 						},
	// 					},
	// 					{
	// 						Type: "reply",
	// 						Reply: &models.ButtonReply{
	// 							ID:    "help_contact",
	// 							Title: "Contact Us",
	// 						},
	// 					},
	// 				},
	// 			},
	// 		}

	// 		if err := wc.whatsappService.SendInteractiveMessage(message.From, interactive); err != nil {
	// 			log.Printf("Failed to send interactive message: %v", err)
	// 		}
	// 		return
	// 	}
	// }

	// Fallback: normal handling (like buttons or text responses)
	// var response string
	// switch message.Type {
	// case "interactive":
	// 	if message.Interactive != nil && message.Interactive.ButtonReply != nil {
	// 		switch message.Interactive.ButtonReply.ID {
	// 		case "help_plant_care": 
	// 			response = "🌱 We can help you with plant care tips."
	// 		case "help_landscaping":
	// 			response = "🌿 Let’s discuss landscaping ideas."
	// 		case "help_contact":
	// 			response = "📞 Contact us at +91-98765-43210."
	// 		default:
	// 			response = "❓ Sorry, I didn’t understand that option."
	// 		}
	// 	}
	// }

	// if response != "" {
	// 	_ = wc.whatsappService.SendTextMessage(message.From, response)
	// }

    // if message.Type == "interactive" && message.Interactive != nil && message.Interactive.ButtonReply != nil {
	// 	buttonID := message.Interactive.ButtonReply.ID
	// 	var apiResponse string

	// 	// Example: Call another API based on button ID
	// 	switch buttonID {
	// 	case "help_plant_care":
	// 		apiResponse = wc.callExternalAPI("https://jsonplaceholder.typicode.com/posts")
	// 	case "help_landscaping":
	// 		apiResponse = wc.callExternalAPI("https://example.com/api/landscaping")
	// 	case "help_contact":
	// 		apiResponse = wc.callExternalAPI("https://example.com/api/contact")
	// 	default:
	// 		apiResponse = "❓ Sorry, I didn’t understand that option."
	// 	}
    //     log.Println("apiResponse: ", apiResponse)
	// 	if apiResponse != "" {
	// 		_ = wc.whatsappService.SendTextMessage(message.From, apiResponse)
	// 	}
	// }


	// Fallback: handle interactive responses menu list
    // var response string
    // if message.Type == "interactive" && message.Interactive != nil {
    //     // Handle button replies
    //     if message.Interactive.ButtonReply != nil {
    //         switch message.Interactive.ButtonReply.ID {
    //         case "help_plant_care":
    //             response = "🌱 We can help you with plant care tips."

    //         case "help_landscaping":
    //             // Show LIST menu instead of simple text
    //             listMenu := &models.InteractiveMessage{
    //                 Type: "list",
    //                 Header: &models.MessageHeader{
    //                     Type: "text",
    //                     Text: "Landscaping Services",
    //                 },
    //                 Body: &models.InteractiveBody{
    //                     Text: "Please select an option:",
    //                 },
    //                 Footer: &models.InteractiveFooter{
    //                     Text: "Tap to choose",
    //                 },
    //                 Action: &models.InteractiveAction{
    //                     Button: "View Options",
    //                     Sections: []models.Section{
    //                         {
    //                             Title: "Landscaping Menu",
    //                             Rows: []models.ListItem{
    //                                 {ID: "design_garden", Title: "Garden Design"},
    //                                 {ID: "install_lawn", Title: "Install Lawn"},
    //                                 {ID: "outdoor_lighting", Title: "Outdoor Lighting"},
    //                                 {ID: "water_features", Title: "Water Features"},
    //                             },
    //                         },
    //                     },
    //                 },
    //             }

    //             if err := wc.whatsappService.SendInteractiveMessage(message.From, listMenu); err != nil {
    //                 log.Printf("Failed to send list menu: %v", err)
    //             }
    //             return // stop here, don’t send plain text

    //         case "help_contact":
    //             response = "📞 Contact us at +91-98765-43210."

    //         default:
    //             response = "❓ Sorry, I didn’t understand that option."
    //         }
    //     }

    //     // Handle list menu replies
    //     if message.Interactive.ListReply != nil {
    //         switch message.Interactive.ListReply.ID {
    //         case "design_garden":
    //             response = "🌷 Great choice! Our garden design team will help create a beautiful outdoor space."
    //         case "install_lawn":
    //             response = "🌱 We can install natural or artificial lawns. Which one would you prefer?"
    //         case "outdoor_lighting":
    //             response = "💡 Outdoor lighting options include solar, LED, and decorative fixtures."
    //         case "water_features":
    //             response = "💦 We can add fountains, ponds, or waterfalls to your landscape."
    //         default:
    //             response = "❓ Sorry, I didn’t understand that option."
    //         }
    //     }
    // }

	//  // Send text fallback if response is set
    // if response != "" {
    //     _ = wc.whatsappService.SendTextMessage(message.From, response)
    // }
}

// ========================
// Appointment API Call
// ========================
func (wc *WhatsAppController) fetchAppointments(ctx context.Context, phone string) ([]Appointment, error) {
    // Example: GET request to your external API
    // req, err := http.NewRequestWithContext(ctx, "GET", "https://jsonplaceholder.typicode.com/posts", nil)
    // if err != nil {
    //     return nil, err
    // }

    // resp, err := http.DefaultClient.Do(req)
    // if err != nil {
    //     return nil, err
    // }
    // defer resp.Body.Close()

    // var raw []map[string]interface{}
    // if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
    //     return nil, err
    // }

    // 🔹 Convert API data into Appointment objects
    // Replace this with your real API fields
    appointments := []Appointment{
        {ID: "appt1", Doctor: "Dr. Smith", Date: "2025-09-01", Time: "10:00 AM"},
        {ID: "appt2", Doctor: "Dr. Johnson", Date: "2025-09-02", Time: "3:00 PM"},
    }

    return appointments, nil
}

// ========================
// Send List of Appointments
// ========================
func (wc *WhatsAppController) sendAppointmentsList(to string, appointments []Appointment) error {
    rows := make([]models.ListItem, 0, len(appointments))

    for _, appt := range appointments {
        rows = append(rows, models.ListItem{
            ID:    appt.ID,
            Title: fmt.Sprintf("%s (%s)", appt.Doctor, appt.Date),
            Description: fmt.Sprintf("Time: %s", appt.Time),
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
func (wc *WhatsAppController) getAppointmentDetails(apptID string) string {
    switch apptID {
    case "appt1":
        return "✅ Appointment with Dr. Smith\n📅 Date: 2025-09-01\n⏰ Time: 10:00 AM"
    case "appt2":
        return "✅ Appointment with Dr. Johnson\n📅 Date: 2025-09-02\n⏰ Time: 3:00 PM"
    }
    return ""
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

// func (wc *WhatsAppController) handleIncomingMessage(ctx context.Context, message models.WhatsAppMessage, metadata models.WhatsAppMetadata) {
//     log.Println("message", message)

//     response := map[string]interface{}{
//         "status":  "success",
//         "message": "Hello from WhatsApp Webhook!",
//         "from":    message.From,
//         "id":      message.ID,
//     }

//     log.Println("response: ", response)

//     jsonBytes, err := json.Marshal(response)
//     if err != nil {
//         log.Printf("Error marshaling JSON: %v", err)
//         return
//     }

//     log.Println("josn response", string(jsonBytes))

//     // Send the JSON string back as a WhatsApp text message
//     wc.whatsappService.SendTextMessage(message.From, string(jsonBytes))
// }

// handleIncomingMessage processes a single incoming message
// func (wc *WhatsAppController) handleIncomingMessage(ctx context.Context, message models.WhatsAppMessage, metadata models.WhatsAppMetadata) {
//     var messageText string
//     var isInteractive bool

//     log.Println("message", message)

//     // Extract message content based on type
//     switch message.Type {
//     case "text":
//         if message.Text != nil {
//             messageText = message.Text.Body
//         }

//     case "interactive":
//         isInteractive = true
//         if message.Interactive != nil {
//             if message.Interactive.ListReply != nil {
//                 messageText = message.Interactive.ListReply.ID
//             } else if message.Interactive.ButtonReply != nil {
//                 messageText = message.Interactive.ButtonReply.ID
//             }
//         }

//     case "button":
//         if message.Button != nil {
//             messageText = message.Button.ID
//         }

//     default:
//         // Send unsupported message type response
//         wc.whatsappService.SendTextMessage(message.From,
//             "Sorry, I can only process text and interactive messages at the moment.")
//         return
//     }

//     // Create chat request
//     chatRequest := models.ChatRequest{
//         Message:   messageText,
//         SessionID: fmt.Sprintf("whatsapp_%s", message.From),
//         UserID:    message.From,
//         Channel:   models.ChannelWhatsApp,
//         Metadata: map[string]interface{}{
//             "whatsapp_message_id": message.ID,
//             "timestamp":          message.Timestamp,
//             "phone_number":       message.From,
//             "is_interactive":     isInteractive,
//         },
//     }

//     // Process through chatbot service
//     response, err := wc.chatbotService.ProcessMessage(ctx, chatRequest)
//     if err != nil {
//         wc.whatsappService.SendTextMessage(message.From,
//             "Sorry, I couldn't process your message. Please try again or contact our support.")
//         return
//     }

//     // Send response
//     if err := wc.sendResponse(message.From, response); err != nil {
//         // Log error but don't send error message to avoid loops
//         fmt.Printf("Failed to send WhatsApp response: %v\n", err)
//     }
// }

// sendResponse sends the chatbot response via WhatsApp
func (wc *WhatsAppController) sendResponse(to string, response *models.ChatResponse) error {
	// Check if we need to send an interactive message
	if response.NeedsInteractiveFormat() {
		return wc.sendInteractiveResponse(to, response)
	}

	// Send simple text message
	return wc.whatsappService.SendTextMessage(to, response.Response)
}

// sendInteractiveResponse sends an interactive WhatsApp message
func (wc *WhatsAppController) sendInteractiveResponse(to string, response *models.ChatResponse) error {
	// If there are actions, convert them to WhatsApp format
	if len(response.Actions) > 0 {
		// Determine if we should use buttons or list based on number of actions
		if len(response.Actions) <= 3 {
			return wc.sendButtonMessage(to, response)
		} else {
			return wc.sendListMessage(to, response)
		}
	}

	// If there's a custom interactive message
	if response.Interactive != nil {
		return wc.whatsappService.SendInteractiveMessage(to, response.Interactive)
	}

	// Fallback to text message
	return wc.whatsappService.SendTextMessage(to, response.Response)
}

// sendButtonMessage sends a button-style interactive message
func (wc *WhatsAppController) sendButtonMessage(to string, response *models.ChatResponse) error {
	buttons := make([]models.InteractiveButton, 0, len(response.Actions))

	for i, action := range response.Actions {
		if i < 3 { // WhatsApp supports max 3 buttons
			buttons = append(buttons, action.ToWhatsAppButton())
		}
	}

	interactive := &models.InteractiveMessage{
		Type: "button",
		Body: &models.InteractiveBody{
			Text: response.Response, // ✅ must be object
		},
		Action: &models.InteractiveAction{
			Buttons: buttons,
		},
	}

	return wc.whatsappService.SendInteractiveMessage(to, interactive)
}

// sendListMessage sends a list-style interactive message
func (wc *WhatsAppController) sendListMessage(to string, response *models.ChatResponse) error {
	rows := make([]models.ListItem, 0, len(response.Actions))

	for _, action := range response.Actions {
		rows = append(rows, action.ToWhatsAppListItem())
	}

	// Group actions into sections if needed
	sections := []models.Section{
		{
			Title: "Options",
			Rows:  rows,
		},
	}

	interactive := &models.InteractiveMessage{
		Type: "list",
		Header: &models.MessageHeader{
			Type: "text",
			Text: "Please select an option",
		},
		Body: &models.InteractiveBody{
			Text: response.Response, // ✅ must be object
		},
		Action: &models.InteractiveAction{
			Button:   "Choose",
			Sections: sections,
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
