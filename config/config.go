package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
    // Server
    Port        string
    Environment string
    
    // Database
    Database DatabaseConfig
    
    // AI Service
    AI AIConfig
    
    // JWT
    JWT JWTConfig
    
    // Email Service
    Email EmailConfig
    
    // SMS Service
    SMS SMSConfig
    
    // File Storage
    Storage StorageConfig
    
    // Security
    Security SecurityConfig
}

type DatabaseConfig struct {
    Type     string // "mongodb" or "postgresql"
    URI      string
    Name     string
    Host     string
    Port     string
    Username string
    Password string
    
    // Connection pool settings
    MaxConnections int
    MinConnections int
    MaxIdleTime    time.Duration
}

type AIConfig struct {
    Provider  string // "gemini", "openai", etc.
    APIKey    string
    Model     string
    MaxTokens int
    Timeout   time.Duration
}

type JWTConfig struct {
    Secret           string
    ExpirationHours  int
    RefreshSecret    string
    RefreshExpDays   int
}

type EmailConfig struct {
    Provider     string // "smtp", "sendgrid", "ses"
    SMTPHost     string
    SMTPPort     int
    Username     string
    Password     string
    FromEmail    string
    FromName     string
    APIKey       string // For services like SendGrid
}

type SMSConfig struct {
    Provider    string // "twilio", "nexmo"
    AccountSID  string
    AuthToken   string
    FromNumber  string
}

type StorageConfig struct {
    Type      string // "local", "s3"
    LocalPath string
    
    // S3 Config
    BucketName      string
    Region          string
    AccessKeyID     string
    SecretAccessKey string
}

type SecurityConfig struct {
    BcryptCost       int
    RateLimitPerMin  int
    AllowedOrigins   []string
    TrustedProxies   []string
}

var cfg *Config

// Load initializes the configuration
func Load() error {
    // Load .env file
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, using environment variables")
    }
    
    cfg = &Config{
        Port:        getEnv("PORT", "8080"),
        Environment: getEnv("ENVIRONMENT", "development"),
        
        Database: DatabaseConfig{
            Type:     getEnv("DB_TYPE", "mongodb"),
            URI:      getEnv("DATABASE_URL", ""),
            Name:     getEnv("DB_NAME", "clinic_chatbot"),
            Host:     getEnv("DB_HOST", "localhost"),
            Port:     getEnv("DB_PORT", "27017"),
            Username: getEnv("DB_USERNAME", ""),
            Password: getEnv("DB_PASSWORD", ""),
            
            MaxConnections: getEnvAsInt("DB_MAX_CONNECTIONS", 100),
            MinConnections: getEnvAsInt("DB_MIN_CONNECTIONS", 10),
            MaxIdleTime:    getEnvAsDuration("DB_MAX_IDLE_TIME", "30m"),
        },
        
        AI: AIConfig{
            Provider:  getEnv("AI_PROVIDER", "gemini"),
            APIKey:    getEnv("GOOGLE_API_KEY", ""),
            Model:     getEnv("AI_MODEL", "gemini-1.5-flash"),
            MaxTokens: getEnvAsInt("AI_MAX_TOKENS", 1000),
            Timeout:   getEnvAsDuration("AI_TIMEOUT", "30s"),
        },
        
        JWT: JWTConfig{
            Secret:          getEnv("JWT_SECRET", ""),
            ExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
            RefreshSecret:   getEnv("JWT_REFRESH_SECRET", ""),
            RefreshExpDays:  getEnvAsInt("JWT_REFRESH_EXP_DAYS", 7),
        },
        
        Email: EmailConfig{
            Provider:  getEnv("EMAIL_PROVIDER", "smtp"),
            SMTPHost:  getEnv("SMTP_HOST", "smtp.gmail.com"),
            SMTPPort:  getEnvAsInt("SMTP_PORT", 587),
            Username:  getEnv("SMTP_USERNAME", ""),
            Password:  getEnv("SMTP_PASSWORD", ""),
            FromEmail: getEnv("EMAIL_FROM", "noreply@clinic.com"),
            FromName:  getEnv("EMAIL_FROM_NAME", "HealthCare Clinic"),
            APIKey:    getEnv("EMAIL_API_KEY", ""),
        },
        
        SMS: SMSConfig{
            Provider:   getEnv("SMS_PROVIDER", "twilio"),
            AccountSID: getEnv("TWILIO_ACCOUNT_SID", ""),
            AuthToken:  getEnv("TWILIO_AUTH_TOKEN", ""),
            FromNumber: getEnv("TWILIO_FROM_NUMBER", ""),
        },
        
        Storage: StorageConfig{
            Type:            getEnv("STORAGE_TYPE", "local"),
            LocalPath:       getEnv("STORAGE_LOCAL_PATH", "./uploads"),
            BucketName:      getEnv("S3_BUCKET_NAME", ""),
            Region:          getEnv("S3_REGION", "us-east-1"),
            AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
            SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
        },
        
        Security: SecurityConfig{
            BcryptCost:      getEnvAsInt("BCRYPT_COST", 10),
            RateLimitPerMin: getEnvAsInt("RATE_LIMIT_PER_MIN", 60),
            AllowedOrigins:  getEnvAsSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
            TrustedProxies:  getEnvAsSlice("TRUSTED_PROXIES", []string{}),
        },
    }
    
    // Validate configuration
    if err := validate(); err != nil {
        return fmt.Errorf("configuration validation failed: %w", err)
    }
    
    return nil
}

// Get returns the loaded configuration
func Get() *Config {
    if cfg == nil {
        log.Fatal("Configuration not loaded. Call Load() first")
    }
    return cfg
}

// Helper functions
func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
    valueStr := getEnv(key, "")
    if value, err := strconv.Atoi(valueStr); err == nil {
        return value
    }
    return defaultValue
}

func getEnvAsDuration(key string, defaultValue string) time.Duration {
    valueStr := getEnv(key, defaultValue)
    if duration, err := time.ParseDuration(valueStr); err == nil {
        return duration
    }
    duration, _ := time.ParseDuration(defaultValue)
    return duration
}

func getEnvAsSlice(key string, defaultValue []string) []string {
    value := getEnv(key, "")
    if value == "" {
        return defaultValue
    }
    // Simple comma-separated parsing
    return strings.Split(value, ",")
}

func validate() error {
    // Validate required fields
    if cfg.Database.Type == "mongodb" && cfg.Database.URI == "" {
        if cfg.Database.Host == "" || cfg.Database.Port == "" {
            return fmt.Errorf("database URI or host/port must be provided")
        }
    }
    
    if cfg.AI.APIKey == "" {
        return fmt.Errorf("AI API key is required")
    }
    
    if cfg.JWT.Secret == "" || cfg.JWT.RefreshSecret == "" {
        return fmt.Errorf("JWT secrets are required")
    }
    
    return nil
}

// BuildDatabaseURI constructs the database URI if not provided
func (c *Config) BuildDatabaseURI() string {
    if c.Database.URI != "" {
        return c.Database.URI
    }
    
    switch c.Database.Type {
    case "mongodb":
        if c.Database.Username != "" && c.Database.Password != "" {
            return fmt.Sprintf("mongodb://%s:%s@%s:%s/%s",
                c.Database.Username,
                c.Database.Password,
                c.Database.Host,
                c.Database.Port,
                c.Database.Name,
            )
        }
        return fmt.Sprintf("mongodb://%s:%s/%s",
            c.Database.Host,
            c.Database.Port,
            c.Database.Name,
        )
    case "postgresql":
        return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
            c.Database.Username,
            c.Database.Password,
            c.Database.Host,
            c.Database.Port,
            c.Database.Name,
        )
    default:
        return ""
    }
}
