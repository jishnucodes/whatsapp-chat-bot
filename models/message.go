package models

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type MessageIntent string

const (
    IntentAppointment    MessageIntent = "appointment"
    IntentMedicalQuery   MessageIntent = "medical_query"
    IntentClinicInfo     MessageIntent = "clinic_info"
    IntentEmergency      MessageIntent = "emergency"
    IntentGreeting       MessageIntent = "greeting"
    IntentUnknown        MessageIntent = "unknown"
)

// MessageChannel represents the communication channel
type MessageChannel string

const (
    ChannelWeb      MessageChannel = "web"
    ChannelWhatsApp MessageChannel = "whatsapp"
)

// Update Message struct to include channel information
type Message struct {
    ID           primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
    SessionID    string                `bson:"session_id" json:"session_id"`
    UserMessage  string                `bson:"user_message" json:"user_message"`
    BotResponse  string                `bson:"bot_response" json:"bot_response"`
    Intent       MessageIntent         `bson:"intent" json:"intent"`
    IsAIResponse bool                  `bson:"is_ai_response" json:"is_ai_response"`
    Timestamp    time.Time             `bson:"timestamp" json:"timestamp"`
    UserID       string                `bson:"user_id,omitempty" json:"user_id,omitempty"`
    Channel      MessageChannel        `bson:"channel,omitempty" json:"channel,omitempty"`
    Metadata     map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

// Update ChatRequest to include channel information
type ChatRequest struct {
    Message   string                 `json:"message" binding:"required"`
    SessionID string                 `json:"session_id" binding:"required"`
    UserID    string                 `json:"user_id,omitempty"`
    Channel   MessageChannel         `json:"channel,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Update ChatResponse to support different response types
type ChatResponse struct {
    Response     string                 `json:"response"`
    Intent       MessageIntent          `json:"intent"`
    Actions      []Action               `json:"actions,omitempty"`
    Data         map[string]interface{} `json:"data,omitempty"`
    ResponseType ResponseType           `json:"response_type,omitempty"`
    Interactive  *InteractiveMessage    `json:"interactive,omitempty"`
}

// ResponseType for different message types
type ResponseType string

const (
    ResponseTypeText        ResponseType = "text"
    ResponseTypeInteractive ResponseType = "interactive"
    ResponseTypeButton      ResponseType = "button"
    ResponseTypeList        ResponseType = "list"
)

// Update Action to support WhatsApp interactive elements
type Action struct {
    Type        string                 `json:"type"`
    Label       string                 `json:"label"`
    Payload     map[string]interface{} `json:"payload,omitempty"`
    Description string                 `json:"description,omitempty"`
    ID          string                 `json:"id,omitempty"`
}

// InteractiveMessage for WhatsApp interactive messages
type InteractiveMessage struct {
    Type    string              `json:"type"` // "list" or "button"
    Header  *MessageHeader      `json:"header,omitempty"`
    Body    string              `json:"body"`
    Footer  string              `json:"footer,omitempty"`
    Action  *InteractiveAction  `json:"action"`
}

type MessageHeader struct {
    Type  string `json:"type"` // "text", "image", "video", "document"
    Text  string `json:"text,omitempty"`
    Media *Media `json:"media,omitempty"`
}

type Media struct {
    Link    string `json:"link,omitempty"`
    Caption string `json:"caption,omitempty"`
}

type InteractiveAction struct {
    Buttons  []InteractiveButton  `json:"buttons,omitempty"`
    Button   string              `json:"button,omitempty"` // For list messages
    Sections []Section           `json:"sections,omitempty"`
}

type InteractiveButton struct {
    Type  string       `json:"type"` // "reply"
    Reply *ButtonReply `json:"reply"`
}

type ButtonReply struct {
    ID    string `json:"id"`
    Title string `json:"title"`
}

type Section struct {
    Title string     `json:"title,omitempty"`
    Rows  []ListItem `json:"rows"`
}

type ListItem struct {
    ID          string `json:"id"`
    Title       string `json:"title"`
    Description string `json:"description,omitempty"`
}

// WhatsApp Webhook Models
type WhatsAppWebhookData struct {
    Object string           `json:"object"`
    Entry  []WhatsAppEntry  `json:"entry"`
}

type WhatsAppEntry struct {
    ID      string            `json:"id"`
    Changes []WhatsAppChange  `json:"changes"`
}

type WhatsAppChange struct {
    Field string         `json:"field"`
    Value WhatsAppValue  `json:"value"`
}

type WhatsAppValue struct {
    MessagingProduct string              `json:"messaging_product"`
    Metadata         WhatsAppMetadata    `json:"metadata"`
    Messages         []WhatsAppMessage   `json:"messages,omitempty"`
    Statuses         []WhatsAppStatus    `json:"statuses,omitempty"`
    Contacts         []WhatsAppContact   `json:"contacts,omitempty"`
}

type WhatsAppMetadata struct {
    DisplayPhoneNumber string `json:"display_phone_number"`
    PhoneNumberID      string `json:"phone_number_id"`
}

type WhatsAppMessage struct {
    From        string                     `json:"from"`
    ID          string                     `json:"id"`
    Timestamp   string                     `json:"timestamp"`
    Type        string                     `json:"type"`
    Text        *WhatsAppText              `json:"text,omitempty"`
    Interactive *WhatsAppInteractiveReply  `json:"interactive,omitempty"`
    Button      *WhatsAppButtonReply       `json:"button,omitempty"`
}

type WhatsAppText struct {
    Body string `json:"body"`
}

type WhatsAppInteractiveReply struct {
    Type        string               `json:"type"`
    ListReply   *WhatsAppListReply   `json:"list_reply,omitempty"`
    ButtonReply *WhatsAppButtonReply `json:"button_reply,omitempty"`
}

type WhatsAppListReply struct {
    ID          string `json:"id"`
    Title       string `json:"title"`
    Description string `json:"description,omitempty"`
}

type WhatsAppButtonReply struct {
    ID    string `json:"id"`
    Title string `json:"title"`
}

type WhatsAppContact struct {
    Profile WhatsAppProfile `json:"profile"`
    WaID    string         `json:"wa_id"`
}

type WhatsAppProfile struct {
    Name string `json:"name"`
}

type WhatsAppStatus struct {
    ID          string    `json:"id"`
    RecipientID string    `json:"recipient_id"`
    Status      string    `json:"status"`
    Timestamp   string    `json:"timestamp"`
    Errors      []Error   `json:"errors,omitempty"`
}

type Error struct {
    Code    int    `json:"code"`
    Title   string `json:"title"`
    Message string `json:"message"`
}

// WhatsApp Send Message Models
type WhatsAppSendMessage struct {
    MessagingProduct string              `json:"messaging_product"`
    RecipientType    string              `json:"recipient_type"`
    To               string              `json:"to"`
    Type             string              `json:"type"`
    Text             *WhatsAppText       `json:"text,omitempty"`
    Interactive      *InteractiveMessage `json:"interactive,omitempty"`
}

// Service Status Model
type WhatsAppServiceStatus struct {
    Enabled             bool      `json:"enabled"`
    WebhookVerified     bool      `json:"webhook_verified"`
    LastMessageReceived time.Time `json:"last_message_received"`
    MessageCountToday   int       `json:"message_count_today"`
    ActiveSessions      int       `json:"active_sessions"`
}

// Session Model for managing conversation state
type ConversationSession struct {
    ID               primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
    SessionID        string                `bson:"session_id" json:"session_id"`
    UserID           string                `bson:"user_id" json:"user_id"`
    Channel          MessageChannel        `bson:"channel" json:"channel"`
    State            string                `bson:"state" json:"state"`
    Context          map[string]interface{} `bson:"context" json:"context"`
    LastActivity     time.Time             `bson:"last_activity" json:"last_activity"`
    CreatedAt        time.Time             `bson:"created_at" json:"created_at"`
    ExpiresAt        time.Time             `bson:"expires_at" json:"expires_at"`
}

// Appointment-related models for WhatsApp interactions
type AppointmentSlot struct {
    Date      string    `json:"date"`
    Time      string    `json:"time"`
    Available bool      `json:"available"`
    DoctorID  string    `json:"doctor_id,omitempty"`
}

type ServiceInfo struct {
    ID          string  `json:"id"`
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Duration    int     `json:"duration"` // in minutes
    Price       float64 `json:"price"`
}

// Helper function to convert Action to WhatsApp format
func (a Action) ToWhatsAppButton() InteractiveButton {
    return InteractiveButton{
        Type: "reply",
        Reply: &ButtonReply{
            ID:    a.ID,
            Title: a.Label,
        },
    }
}

// Helper function to convert Action to WhatsApp list item
func (a Action) ToWhatsAppListItem() ListItem {
    return ListItem{
        ID:          a.ID,
        Title:       a.Label,
        Description: a.Description,
    }
}

// Helper method to check if response needs interactive formatting
func (cr ChatResponse) NeedsInteractiveFormat() bool {
    return len(cr.Actions) > 0 || cr.ResponseType == ResponseTypeInteractive
}

// Helper method to create a simple text response
func NewTextResponse(text string, intent MessageIntent) *ChatResponse {
    return &ChatResponse{
        Response:     text,
        Intent:       intent,
        ResponseType: ResponseTypeText,
    }
}

// Helper method to create an interactive response
func NewInteractiveResponse(text string, intent MessageIntent, actions []Action) *ChatResponse {
    return &ChatResponse{
        Response:     text,
        Intent:       intent,
        Actions:      actions,
        ResponseType: ResponseTypeInteractive,
    }
}
