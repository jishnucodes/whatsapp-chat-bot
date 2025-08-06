package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/gin-gonic/gin"
    "clinic-chatbot-backend/config"
    "clinic-chatbot-backend/database"
    "clinic-chatbot-backend/routes"
)

func main() {
    // Load configuration
    if err := config.Load(); err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    cfg := config.Get()
    
    // Set Gin mode
    if cfg.Environment == "production" {
        gin.SetMode(gin.ReleaseMode)
    }
    
    // Connect to database
    if err := database.Connect(cfg); err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer database.Disconnect()
    
    // Verify WhatsApp configuration
    if err := verifyWhatsAppConfig(); err != nil {
        log.Printf("WARNING: WhatsApp integration may not work properly: %v", err)
        // Continue running without WhatsApp if not configured
    } else {
        log.Println("WhatsApp configuration verified successfully")
    }
    
    // Create Gin router
    router := gin.New()
    
    // Add middleware
    router.Use(gin.Recovery())
    router.Use(gin.Logger())

    // CORS middleware
    router.Use(func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
        c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    })
    
    // Health check endpoint
    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "status": "ok",
            "timestamp": time.Now(),
            "whatsapp_configured": os.Getenv("WHATSAPP_ACCESS_TOKEN") != "",
        })
    })
    
    // Setup all routes
    routes.SetupRoutes(router)
    
    // Log available endpoints
    logAvailableEndpoints(router)
    
    // Create HTTP server
    srv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      router,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    // Start server in a goroutine
    go func() {
        log.Printf("Server starting on port %s", cfg.Port)
        log.Printf("Health check: http://localhost:%s/health", cfg.Port)
        log.Printf("WhatsApp webhook URL: http://localhost:%s/api/whatsapp/webhook", cfg.Port)
        
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Failed to start server: %v", err)
        }
    }()
    
    // Wait for interrupt signal to gracefully shutdown the server
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Println("Shutting down server...")
    
    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("Server forced to shutdown: %v", err)
    }
    
    log.Println("Server exited")
}

// verifyWhatsAppConfig checks if WhatsApp configuration is present
func verifyWhatsAppConfig() error {
    required := []string{
        "WHATSAPP_ACCESS_TOKEN",
        "WHATSAPP_PHONE_NUMBER_ID",
        "WHATSAPP_VERIFY_TOKEN",
    }
    
    missing := []string{}
    for _, key := range required {
        if os.Getenv(key) == "" {
            missing = append(missing, key)
        }
    }
    
    if len(missing) > 0 {
        return fmt.Errorf("missing required environment variables: %v", missing)
    }
    
    return nil
}

// logAvailableEndpoints logs all registered routes
func logAvailableEndpoints(router *gin.Engine) {
    log.Println("\nAvailable endpoints:")
    routes := router.Routes()
    for _, route := range routes {
        log.Printf("  %s %s", route.Method, route.Path)
    }
    log.Println("")
}
