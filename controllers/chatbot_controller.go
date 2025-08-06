package controllers

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "clinic-chatbot-backend/models"
    "clinic-chatbot-backend/services"
)

type ChatbotController struct {
    chatbotService *services.ChatbotService
}

func NewChatbotController(chatbotService *services.ChatbotService) *ChatbotController {
    return &ChatbotController{
        chatbotService: chatbotService,
    }
}

// HandleChat processes chat messages
func (cc *ChatbotController) HandleChat(c *gin.Context) {
    var req models.ChatRequest
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request format",
            "details": err.Error(),
        })
        return
    }
    
    // Get user ID from context if authenticated
    userID, _ := c.Get("userID")
    if userID != nil {
        req.UserID = userID.(string)
    }
    
    response, err := cc.chatbotService.ProcessMessage(c.Request.Context(), req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to process message",
            "details": err.Error(),
        })
        return
    }
    
    c.JSON(http.StatusOK, response)
}

// GetChatHistory retrieves chat history for a user
// func (cc *ChatbotController) GetChatHistory(c *gin.Context) {
//     userID := c.GetString("userID")
//     sessionID := c.Query("session_id")
//     limit := 50
    
//     if limitStr := c.Query("limit"); limitStr != "" {
//         if l, err := strconv.Atoi(limitStr); err == nil {
//             limit = l
//         }
//     }
    
//     history, err := cc.chatbotService.GetChatHistory(c.Request.Context(), userID, sessionID, limit)
//     if err != nil {
//         c.JSON(http.StatusInternalServerError, gin.H{
//             "error": "Failed to retrieve chat history",
//         })
//         return
//     }
    
//     c.JSON(http.StatusOK, gin.H{
//         "history": history,
//         "count": len(history),
//     })
// }

// // ClearChatHistory clears chat history
// func (cc *ChatbotController) ClearChatHistory(c *gin.Context) {
//     userID := c.GetString("userID")
//     sessionID := c.Query("session_id")
    
//     err := cc.chatbotService.ClearChatHistory(c.Request.Context(), userID, sessionID)
//     if err != nil {
//         c.JSON(http.StatusInternalServerError, gin.H{
//             "error": "Failed to clear chat history",
//         })
//         return
//     }
    
//     c.JSON(http.StatusOK, gin.H{
//         "message": "Chat history cleared successfully",
//     })
// }

// // GetChatSessions retrieves all chat sessions for a user
// func (cc *ChatbotController) GetChatSessions(c *gin.Context) {
//     userID := c.GetString("userID")
    
//     sessions, err := cc.chatbotService.GetUserSessions(c.Request.Context(), userID)
//     if err != nil {
//         c.JSON(http.StatusInternalServerError, gin.H{
//             "error": "Failed to retrieve sessions",
//         })
//         return
//     }
    
//     c.JSON(http.StatusOK, gin.H{
//         "sessions": sessions,
//     })
// }

// GetSupportedIntents returns list of supported intents
func (cc *ChatbotController) GetSupportedIntents(c *gin.Context) {
    intents := []map[string]interface{}{
        {
            "intent": "appointment",
            "description": "Book, cancel, or reschedule appointments",
            "examples": []string{
                "I want to book an appointment",
                "Cancel my appointment",
                "Reschedule my visit",
            },
        },
        {
            "intent": "medical_query",
            "description": "General health and medical questions",
            "examples": []string{
                "What are the symptoms of flu?",
                "How to treat a headache?",
                "Is fever dangerous?",
            },
        },
        {
            "intent": "clinic_info",
            "description": "Information about clinic services, hours, location",
            "examples": []string{
                "What are your opening hours?",
                "Where is the clinic located?",
                "What services do you offer?",
            },
        },
        {
            "intent": "emergency",
            "description": "Emergency assistance",
            "examples": []string{
                "I need emergency help",
                "Severe chest pain",
                "Can't breathe properly",
            },
        },
    }
    
    c.JSON(http.StatusOK, gin.H{
        "intents": intents,
    })
}

// GetClinicInfo returns clinic information
// func (cc *ChatbotController) GetClinicInfo(c *gin.Context) {
//     info := cc.chatbotService.GetClinicInfo()
//     c.JSON(http.StatusOK, info)
// }

// GetServices returns available services
// func (cc *ChatbotController) GetServices(c *gin.Context) {
//     services := cc.chatbotService.GetAvailableServices()
//     c.JSON(http.StatusOK, gin.H{
//         "services": services,
//     })
// }

// GetChatAnalytics returns chat analytics (admin only)
// func (cc *ChatbotController) GetChatAnalytics(c *gin.Context) {
//     startDate := c.Query("start_date")
//     endDate := c.Query("end_date")
    
//     if startDate == "" {
//         startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
//     }
//     if endDate == "" {
//         endDate = time.Now().Format("2006-01-02")
//     }
    
//     analytics, err := cc.chatbotService.GetAnalytics(c.Request.Context(), startDate, endDate)
//     if err != nil {
//         c.JSON(http.StatusInternalServerError, gin.H{
//             "error": "Failed to retrieve analytics",
//         })
//         return
//     }
    
//     c.JSON(http.StatusOK, analytics)
// }

// GetAllMessages retrieves all messages (admin only)
// func (cc *ChatbotController) GetAllMessages(c *gin.Context) {
//     page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
//     limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
    
//     messages, total, err := cc.chatbotService.GetAllMessages(c.Request.Context(), page, limit)
//     if err != nil {
//         c.JSON(http.StatusInternalServerError, gin.H{
//             "error": "Failed to retrieve messages",
//         })
//         return
//     }
    
//     c.JSON(http.StatusOK, gin.H{
//         "messages": messages,
//         "total": total,
//         "page": page,
//         "limit": limit,
//         "total_pages": (total + limit - 1) / limit,
//     })
// }
