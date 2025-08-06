package routes

import (
    "github.com/gin-gonic/gin"
    "clinic-chatbot-backend/controllers"
    "clinic-chatbot-backend/services"
    // "clinic-chatbot-backend/middleware"
    // "clinic-chatbot-backend/database"
    // "clinic-chatbot-backend/config"
)

func SetupRoutes(router *gin.Engine) {
    // Initialize services
    aiService := services.NewAIService()
    chatbotService := services.NewChatbotService(aiService)
    whatsappService := services.NewWhatsAppService()
    
    // Initialize controllers
    chatbotController := controllers.NewChatbotController(chatbotService)
    wsController := controllers.NewWebSocketController(chatbotService)
    whatsappController := controllers.NewWhatsAppController(whatsappService, chatbotService)
    
    // Public routes (no authentication required)
    public := router.Group("/api/v1")
    {
        // Chatbot (basic access)
        public.POST("/chat", chatbotController.HandleChat)
        
        // WebSocket for real-time chat
        public.GET("/ws", wsController.HandleWebSocket)
    }
    
    // WhatsApp routes
    whatsapp := router.Group("/api/whatsapp")
    {
        // Webhook endpoints (no auth required for WhatsApp to call)
        whatsapp.GET("/webhook", whatsappController.VerifyWebhook)
        whatsapp.POST("/webhook", whatsappController.HandleWebhook)
        
        // Admin endpoints (require auth) - commented out for now since middleware isn't available
        // admin := whatsapp.Group("/admin")
        // admin.Use(middleware.RequireAuth())
        // {
        //     admin.POST("/send", whatsappController.SendMessage)
        //     admin.GET("/status", whatsappController.GetStatus)
        // }
        
        // Temporarily add admin routes without auth for testing
        whatsapp.POST("/admin/send", whatsappController.SendMessage)
        whatsapp.GET("/admin/status", whatsappController.GetStatus)
    }
    
    // Static files (if serving from Go)
    router.Static("/uploads", "./uploads")
    
    // 404 handler
    router.NoRoute(func(c *gin.Context) {
        c.JSON(404, gin.H{
            "error": "Route not found",
            "path": c.Request.URL.Path,
        })
    })
}
