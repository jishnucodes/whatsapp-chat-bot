package database

import (
	"context"
	"fmt"
	"time"

	"clinic-chatbot-backend/config"

	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Connect establishes database connection based on config
func Connect(cfg *config.Config) error {
    switch cfg.Database.Type {
    case "mongodb":
        return ConnectMongoDB(cfg)
    default:
        return fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
    }
}

// Disconnect closes database connection
func Disconnect() error {
    cfg := config.Get()
    
    switch cfg.Database.Type {
    case "mongodb":
        return DisconnectMongoDB()
    default:
        return nil
    }
}

// HealthCheck performs a database health check
func HealthCheck() error {
    cfg := config.Get()
    
    switch cfg.Database.Type {
    case "mongodb":
        client := GetMongoClient()
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return client.Ping(ctx, readpref.Primary())
        
    // case "postgresql":
    //     db := GetPostgresDB()
    //     return db.Ping()
        
    default:
        return fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
    }
}
