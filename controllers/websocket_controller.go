package controllers

import (
	"clinic-chatbot-backend/models"
	"clinic-chatbot-backend/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // Configure properly for production
    },
}

type WebSocketController struct {
    chatbotService *services.ChatbotService
}

func NewWebSocketController(chatbotService *services.ChatbotService) *WebSocketController {
    return &WebSocketController{
        chatbotService: chatbotService,
    }
}

func (wc *WebSocketController) HandleWebSocket(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        log.Println("WebSocket upgrade error:", err)
        return
    }
    defer conn.Close()
    
    sessionID := c.Query("session_id")
    // if sessionID == "" {
    //     sessionID = generateSessionID() // Implement this
    // }
    
    for {
        var msg map[string]string
        err := conn.ReadJSON(&msg)
        if err != nil {
            log.Println("Read error:", err)
            break
        }
        
        req := models.ChatRequest{
            Message:   msg["message"],
            SessionID: sessionID,
            UserID:    msg["user_id"],
        }
        
        response, err := wc.chatbotService.ProcessMessage(c.Request.Context(), req)
        if err != nil {
            conn.WriteJSON(map[string]interface{}{
                "error": "Failed to process message",
            })
            continue
        }
        
        conn.WriteJSON(response)
    }
}
