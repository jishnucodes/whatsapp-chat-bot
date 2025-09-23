// services/whatsapp_service.go
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"clinic-chatbot-backend/models"
)

type WhatsAppService struct {
    apiURL          string
    apiVersion      string
    accessToken     string
    phoneNumberID   string
    businessID      string
    verifyToken     string
    httpClient      *http.Client
    
    // Status tracking
    statusMu        sync.RWMutex
    lastMessageTime time.Time
    messageCount    int64
    dailyCount      map[string]int
}

func NewWhatsAppService() *WhatsAppService {
    return &WhatsAppService{
        apiURL:        "https://graph.facebook.com",
        apiVersion: "v18.0",
        accessToken:   os.Getenv("WHATSAPP_ACCESS_TOKEN"),
        phoneNumberID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"),
        businessID:    os.Getenv("WHATSAPP_BUSINESS_ID"),
        verifyToken:   os.Getenv("WHATSAPP_VERIFY_TOKEN"),
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        dailyCount: make(map[string]int),
    }
}

// GetVerifyToken returns the webhook verification token
func (ws *WhatsAppService) GetVerifyToken() string {
    log.Println("Verify token: ", ws.verifyToken)
    return ws.verifyToken
}

// SendTextMessage sends a simple text message
func (ws *WhatsAppService) SendTextMessage(to string, message string) error {
    // Clean and validate phone number
    to = ws.CleanPhoneNumber(to)
    
    payload := models.WhatsAppSendMessage{
        MessagingProduct: "whatsapp",
        RecipientType:    "individual",
        To:               to,
        Type:             "text",
        Text: &models.WhatsAppText{
            Body: message,
        },
    }
    
    return ws.sendMessage(payload)
}

// SendInteractiveMessage sends an interactive message
func (ws *WhatsAppService) SendInteractiveMessage(to string, interactive *models.InteractiveMessage) error {
    to = ws.CleanPhoneNumber(to)
    
    payload := models.WhatsAppSendMessage{
        MessagingProduct: "whatsapp",
        RecipientType:    "individual",
        To:               to,
        Type:             "interactive",
        Interactive:      interactive,
    }

    log.Println("payload from whatsapp", payload)
    
    return ws.sendMessage(payload)
}

// SendTemplateMessage sends a template message
func (ws *WhatsAppService) SendTemplateMessage(to string, templateName string, params []string) error {
    to = ws.CleanPhoneNumber(to)
    
    // Build template components
    components := []map[string]interface{}{
        {
            "type": "body",
            "parameters": ws.buildTemplateParams(params),
        },
    }
    
    payload := map[string]interface{}{
        "messaging_product": "whatsapp",
        "recipient_type":    "individual",
        "to":                to,
        "type":              "template",
        "template": map[string]interface{}{
            "name":       templateName,
            "language":   map[string]string{"code": "en"},
            "components": components,
        },
    }
    
    return ws.sendRequest(payload)
}

// buildTemplateParams converts string params to WhatsApp format
func (ws *WhatsAppService) buildTemplateParams(params []string) []map[string]interface{} {
    templateParams := make([]map[string]interface{}, len(params))
    for i, param := range params {
        templateParams[i] = map[string]interface{}{
            "type": "text",
            "text": param,
        }
    }
    return templateParams
}

// sendMessage sends a message via WhatsApp API
func (ws *WhatsAppService) sendMessage(message models.WhatsAppSendMessage) error {
    return ws.sendRequest(message)
}

// Update sendRequest with better error logging
func (ws *WhatsAppService) sendRequest(payload interface{}) error {
    url := fmt.Sprintf("%s/%s/%s/messages", ws.apiURL, ws.apiVersion, ws.phoneNumberID)
    
    // Log the URL being called
    log.Printf("Sending request to: %s", url)
    
    jsonPayload, err := json.Marshal(payload)
    if err != nil {
        log.Printf("Failed to marshal payload: %v", err)
        return fmt.Errorf("failed to marshal payload: %w", err)
    }
    
    // Log the payload being sent
    log.Printf("Request payload: %s", string(jsonPayload))
    
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
    if err != nil {
        log.Printf("Failed to create request: %v", err)
        return fmt.Errorf("failed to create request: %w", err)
    }
    
    // Log headers
    req.Header.Set("Authorization", "Bearer "+ws.accessToken)
    req.Header.Set("Content-Type", "application/json")
    log.Printf("Using access token: %v", ws.accessToken != "")
    
    resp, err := ws.httpClient.Do(req)
    if err != nil {
        log.Printf("Failed to send request: %v", err)
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("Failed to read response: %v", err)
        return fmt.Errorf("failed to read response: %w", err)
    }
    
    // Always log the response
    log.Printf("Response status: %d", resp.StatusCode)
    log.Printf("Response body: %s", string(body))
    
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        var errorResp map[string]interface{}
        if err := json.Unmarshal(body, &errorResp); err == nil {
            log.Printf("WhatsApp API error details: %+v", errorResp)
            
            // Check for specific error codes
            if errData, ok := errorResp["error"].(map[string]interface{}); ok {
                if code, ok := errData["code"].(float64); ok {
                    log.Printf("Error code: %v", code)
                }
                if message, ok := errData["message"].(string); ok {
                    log.Printf("Error message: %s", message)
                }
            }
            return fmt.Errorf("WhatsApp API error: %v", errorResp)
        }
        return fmt.Errorf("WhatsApp API error: %s", string(body))
    }
    
    ws.updateMessageStatus()
    return nil
}

// sendRequest sends HTTP request to WhatsApp API
// func (ws *WhatsAppService) sendRequest(payload interface{}) error {
//     url := fmt.Sprintf("%s/%s/%s/messages", ws.apiURL, ws.apiVersion, ws.phoneNumberID)
    
//     jsonPayload, err := json.Marshal(payload)
//     if err != nil {
//         return fmt.Errorf("failed to marshal payload: %w", err)
//     }
    
//     req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
//     if err != nil {
//         log.Println("failed to create request: %w", err)
//         return fmt.Errorf("failed to create request: %w", err)
//     }
    
//     req.Header.Set("Authorization", "Bearer "+ws.accessToken)
//     req.Header.Set("Content-Type", "application/json")
    
//     resp, err := ws.httpClient.Do(req)
//     if err != nil {
//         log.Println("failed to send request: %w", err)
//         return fmt.Errorf("failed to send request: %w", err)
//     }
//     defer resp.Body.Close()
    
//     body, err := io.ReadAll(resp.Body)
//     if err != nil {
//         log.Println("failed to read response: %w", err)
//         return fmt.Errorf("failed to read response: %w", err)
//     }
    
//     if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
//         var errorResp map[string]interface{}
//         if err := json.Unmarshal(body, &errorResp); err == nil {
//             log.Println("WhatsApp API error: ", errorResp)
//             return fmt.Errorf("WhatsApp API error: %v", errorResp)
//         }
//         return fmt.Errorf("WhatsApp API error: %s", string(body))
//     }
    
//     // Update status
//     ws.updateMessageStatus()
    
//     return nil
// }

// SendMediaMessage sends media (image, document, etc.)
func (ws *WhatsAppService) SendMediaMessage(to string, mediaType string, mediaURL string, caption string) error {
    to = ws.CleanPhoneNumber(to)
    
    payload := map[string]interface{}{
        "messaging_product": "whatsapp",
        "recipient_type":    "individual",
        "to":                to,
        "type":              mediaType,
        mediaType: map[string]interface{}{
            "link":    mediaURL,
            "caption": caption,
        },
    }
    
    return ws.sendRequest(payload)
}

// SendLocationMessage sends a location
func (ws *WhatsAppService) SendLocationMessage(to string, lat, lng float64, name, address string) error {
    to = ws.CleanPhoneNumber(to)
    
    payload := map[string]interface{}{
        "messaging_product": "whatsapp",
        "recipient_type":    "individual",
        "to":                to,
        "type":              "location",
        "location": map[string]interface{}{
            "latitude":  lat,
            "longitude": lng,
            "name":      name,
            "address":   address,
        },
    }
    
    return ws.sendRequest(payload)
}

// SendContactMessage sends a contact
func (ws *WhatsAppService) SendContactMessage(to string, contacts []map[string]interface{}) error {
    to = ws.CleanPhoneNumber(to)
    
    payload := map[string]interface{}{
        "messaging_product": "whatsapp",
        "recipient_type":    "individual",
        "to":                to,
        "type":              "contacts",
        "contacts":          contacts,
    }
    
    return ws.sendRequest(payload)
}

// CleanPhoneNumber cleans and validates phone number
func (ws *WhatsAppService) CleanPhoneNumber(phone string) string {
    // Remove all non-numeric characters
    cleaned := strings.Map(func(r rune) rune {
        if r >= '0' && r <= '9' {
            return r
        }
        return -1
    }, phone)
    
    // Add country code if missing (assuming US for example)
    if len(cleaned) == 10 {
        cleaned = "1" + cleaned
    }
    
    return cleaned
}

// updateMessageStatus updates internal message tracking
func (ws *WhatsAppService) updateMessageStatus() {
    ws.statusMu.Lock()
    defer ws.statusMu.Unlock()
    
    ws.lastMessageTime = time.Now()
    ws.messageCount++
    
    // Update daily count
    today := time.Now().Format("2006-01-02")
    ws.dailyCount[today]++
}

// GetStatus returns the service status
func (ws *WhatsAppService) GetStatus() models.WhatsAppServiceStatus {
    ws.statusMu.RLock()
    defer ws.statusMu.RUnlock()
    
    today := time.Now().Format("2006-01-02")
    
    return models.WhatsAppServiceStatus{
        Enabled:             ws.accessToken != "" && ws.phoneNumberID != "",
        WebhookVerified:     true, // This should be tracked properly
        LastMessageReceived: ws.lastMessageTime,
        MessageCountToday:   ws.dailyCount[today],
        ActiveSessions:      0, // This should come from session manager
    }
}

// MarkMessageAsRead marks a message as read
func (ws *WhatsAppService) MarkMessageAsRead(messageID string) error {
    url := fmt.Sprintf("%s/%s/%s/messages", ws.apiURL, ws.apiVersion, ws.phoneNumberID)
	fmt.Println("url", url)
    
    payload := map[string]interface{}{
        "messaging_product": "whatsapp",
        "status":            "read",
        "message_id":        messageID,
    }
    
    return ws.sendRequest(payload)
}

// GetBusinessProfile retrieves business profile information
func (ws *WhatsAppService) GetBusinessProfile() (map[string]interface{}, error) {
    url := fmt.Sprintf("%s/%s/%s", ws.apiURL, ws.apiVersion, ws.businessID)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Authorization", "Bearer "+ws.accessToken)
    
    resp, err := ws.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var profile map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
        return nil, err
    }
    
    return profile, nil
}

// ValidatePhoneNumber checks if a phone number has WhatsApp
func (ws *WhatsAppService) ValidatePhoneNumber(phoneNumber string) (bool, error) {
    // This would use the WhatsApp Business API to check if number has WhatsApp
    // For now, return true as placeholder
    return true, nil
}
