// controllers/whatsapp_controller.go
package controllers

import (
	"context"
	"io"
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

    return string(body)
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

	// Check if it's a text message
	if message.Type == "text" && message.Text != nil {
		userText := strings.TrimSpace(strings.ToLower(message.Text.Body))
		if userText == "hi" || userText == "hello" {
			// Send interactive buttons instead of plain text
			interactive := &models.InteractiveMessage{
				Type: "button",
				Body: &models.InteractiveBody{
					Text: "Hi! How can we help you today?",
				},
				Footer: &models.InteractiveFooter{
					Text: "Sanitas Landscaping",
				},
				Action: &models.InteractiveAction{
					Buttons: []models.InteractiveButton{
						{
							Type: "reply",
							Reply: &models.ButtonReply{
								ID:    "help_plant_care",
								Title: "Plant Care",
							},
						},
						{
							Type: "reply",
							Reply: &models.ButtonReply{
								ID:    "help_landscaping",
								Title: "Landscaping",
							},
						},
						{
							Type: "reply",
							Reply: &models.ButtonReply{
								ID:    "help_contact",
								Title: "Contact Us",
							},
						},
					},
				},
			}

			if err := wc.whatsappService.SendInteractiveMessage(message.From, interactive); err != nil {
				log.Printf("Failed to send interactive message: %v", err)
			}
			return
		}
	}

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

    if message.Type == "interactive" && message.Interactive != nil && message.Interactive.ButtonReply != nil {
		buttonID := message.Interactive.ButtonReply.ID
		var apiResponse string

		// Example: Call another API based on button ID
		switch buttonID {
		case "help_plant_care":
			apiResponse = wc.callExternalAPI("https://jsonplaceholder.typicode.com/posts")
		case "help_landscaping":
			apiResponse = wc.callExternalAPI("https://example.com/api/landscaping")
		case "help_contact":
			apiResponse = wc.callExternalAPI("https://example.com/api/contact")
		default:
			apiResponse = "❓ Sorry, I didn’t understand that option."
		}
        log.Println("apiResponse: ", apiResponse)
		if apiResponse != "" {
			_ = wc.whatsappService.SendTextMessage(message.From, apiResponse)
		}
	}
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
