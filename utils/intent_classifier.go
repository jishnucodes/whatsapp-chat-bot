package utils

import (
    "strings"
    "clinic-chatbot-backend/models"
)

type IntentClassifier struct {
    patterns map[models.MessageIntent][]string
}

func NewIntentClassifier() *IntentClassifier {
    return &IntentClassifier{
        patterns: map[models.MessageIntent][]string{
            models.IntentAppointment: {
                "appointment", "book", "schedule", "doctor", "consultation",
                "available", "slot", "timing", "visit", "checkup",
            },
            models.IntentMedicalQuery: {
                "symptom", "pain", "medicine", "treatment", "disease",
                "diagnosis", "health", "medical", "condition", "prescription",
                "fever", "cold", "headache", "allergy",
            },
            models.IntentClinicInfo: {
                "clinic", "location", "address", "hours", "timing",
                "contact", "phone", "services", "specialization", "doctor list",
                "facilities", "insurance", "payment",
            },
            models.IntentEmergency: {
                "emergency", "urgent", "immediate", "critical", "severe",
                "accident", "bleeding", "unconscious", "chest pain",
            },
            models.IntentGreeting: {
                "hello", "hi", "hey", "good morning", "good evening",
                "how are you", "greetings",
            },
        },
    }
}

func (ic *IntentClassifier) ClassifyIntent(message string) models.MessageIntent {
    message = strings.ToLower(message)
    
    // Check for emergency keywords first
    if ic.containsAnyKeyword(message, ic.patterns[models.IntentEmergency]) {
        return models.IntentEmergency
    }
    
    // Score each intent
    scores := make(map[models.MessageIntent]int)
    for intent, keywords := range ic.patterns {
        for _, keyword := range keywords {
            if strings.Contains(message, keyword) {
                scores[intent]++
            }
        }
    }
    
    // Find intent with highest score
    var maxIntent models.MessageIntent = models.IntentUnknown
    maxScore := 0
    for intent, score := range scores {
        if score > maxScore {
            maxScore = score
            maxIntent = intent
        }
    }
    
    return maxIntent
}

func (ic *IntentClassifier) containsAnyKeyword(message string, keywords []string) bool {
    for _, keyword := range keywords {
        if strings.Contains(message, keyword) {
            return true
        }
    }
    return false
}
