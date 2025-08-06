package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"clinic-chatbot-backend/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
    mongoClient *mongo.Client
    mongoDB     *mongo.Database
)

// ConnectMongoDB establishes connection to MongoDB
func ConnectMongoDB(cfg *config.Config) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Set client options
    clientOptions := options.Client().
        ApplyURI(cfg.BuildDatabaseURI()).
        SetMaxPoolSize(uint64(cfg.Database.MaxConnections)).
        SetMinPoolSize(uint64(cfg.Database.MinConnections)).
        SetMaxConnIdleTime(cfg.Database.MaxIdleTime)
    
    // Connect to MongoDB
    client, err := mongo.Connect(ctx, clientOptions)
    if err != nil {
        return fmt.Errorf("failed to connect to MongoDB: %w", err)
    }
    
    // Ping the database
    if err := client.Ping(ctx, readpref.Primary()); err != nil {
        return fmt.Errorf("failed to ping MongoDB: %w", err)
    }
    
    mongoClient = client
    mongoDB = client.Database(cfg.Database.Name)
    
    log.Printf("Connected to MongoDB database: %s", cfg.Database.Name)
    
    // Create indexes
    if err := createIndexes(ctx); err != nil {
        return fmt.Errorf("failed to create indexes: %w", err)
    }
    
    return nil
}

// GetMongoDB returns the MongoDB database instance
func GetMongoDB() *mongo.Database {
    if mongoDB == nil {
        log.Fatal("MongoDB not initialized")
    }
    return mongoDB
}

// GetMongoClient returns the MongoDB client
func GetMongoClient() *mongo.Client {
    if mongoClient == nil {
        log.Fatal("MongoDB client not initialized")
    }
    return mongoClient
}

// createIndexes creates necessary indexes
func createIndexes(ctx context.Context) error {
    // Appointments indexes
    appointmentsCollection := mongoDB.Collection("appointments")
    appointmentIndexes := []mongo.IndexModel{
        {
            Keys: bson.D{
                {Key: "patient_id", Value: 1},
                {Key: "start_time", Value: -1},
            },
        },
        {
            Keys: bson.D{
                {Key: "doctor_id", Value: 1},
                {Key: "start_time", Value: 1},
            },
        },
        {
            Keys: bson.D{{Key: "status", Value: 1}},
        },
    }
    
    if _, err := appointmentsCollection.Indexes().CreateMany(ctx, appointmentIndexes); err != nil {
        return fmt.Errorf("failed to create appointment indexes: %w", err)
    }
    
    // Messages indexes
    messagesCollection := mongoDB.Collection("messages")
    messageIndexes := []mongo.IndexModel{
        {
            Keys: bson.D{{Key: "session_id", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "user_id", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "timestamp", Value: -1}},
        },
    }
    
    if _, err := messagesCollection.Indexes().CreateMany(ctx, messageIndexes); err != nil {
        return fmt.Errorf("failed to create message indexes: %w", err)
    }
    
    // Users indexes
    usersCollection := mongoDB.Collection("users")
    userIndexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "email", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{Key: "phone", Value: 1}},
        },
    }
    
    if _, err := usersCollection.Indexes().CreateMany(ctx, userIndexes); err != nil {
        return fmt.Errorf("failed to create user indexes: %w", err)
    }
    
    log.Println("Database indexes created successfully")
    return nil
}

// DisconnectMongoDB closes the MongoDB connection
func DisconnectMongoDB() error {
    if mongoClient == nil {
        return nil
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := mongoClient.Disconnect(ctx); err != nil {
        return fmt.Errorf("failed to disconnect from MongoDB: %w", err)
    }
    
    log.Println("Disconnected from MongoDB")
    return nil
}
