// middleware/whatsapp_verification.go
package middleware

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "io"
    "net/http"
    "os"
    
    "github.com/gin-gonic/gin"
)

func VerifyWhatsAppSignature() gin.HandlerFunc {
    return func(c *gin.Context) {
        signature := c.GetHeader("X-Hub-Signature-256")
        if signature == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing signature"})
            return
        }
        
        // Read the request body
        body, err := io.ReadAll(c.Request.Body)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
            return
        }
        
        // Restore the body for subsequent handlers
        c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
        
        // Calculate expected signature
        appSecret := os.Getenv("WHATSAPP_APP_SECRET")
        expectedSig := calculateHMAC(body, appSecret)
        
        // Compare signatures
        if !hmac.Equal([]byte(signature), []byte("sha256="+expectedSig)) {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
            return
        }
        
        c.Next()
    }
    
}

func calculateHMAC(data []byte, secret string) string {
    h := hmac.New(sha256.New, []byte(secret))
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}
