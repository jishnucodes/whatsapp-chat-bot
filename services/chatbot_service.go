package services

import (
    "context"
    "fmt"
    "strings"
    "time"
    "clinic-chatbot-backend/models"
    "clinic-chatbot-backend/utils"
)

type ChatbotService struct {
    aiService        *AIService
    // appointmentSvc   *AppointmentService
    intentClassifier *utils.IntentClassifier
    clinicInfo       map[string]string
}

func NewChatbotService(aiService *AIService) *ChatbotService {
    return &ChatbotService{
        aiService:        aiService,
        // appointmentSvc:   appointmentSvc,
        intentClassifier: utils.NewIntentClassifier(),
        clinicInfo: map[string]string{
            "name":     "HealthCare Clinic",
            "address":  "123 Medical Center, Downtown",
            "phone":    "+1-234-567-8900",
            "hours":    "Mon-Fri: 9AM-6PM, Sat: 9AM-2PM",
            "services": "General Medicine, Pediatrics, Cardiology, Dermatology",
        },
    }
}

func (s *ChatbotService) ProcessMessage(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
    // Classify intent
    intent := s.intentClassifier.ClassifyIntent(req.Message)
    
    // Create message record
    message := &models.Message{
        SessionID:   req.SessionID,
        UserMessage: req.Message,
        Intent:      intent,
        Timestamp:   time.Now(),
        UserID:      req.UserID,
    }
    
    var response *models.ChatResponse
    var err error
    
    // Handle based on intent
    switch intent {
    case models.IntentEmergency:
        response, err = s.handleEmergency()
    // case models.IntentAppointment:
    //     response, err = s.handleAppointment(req)
    case models.IntentClinicInfo:
        response, err = s.handleClinicInfo(req.Message)
    case models.IntentMedicalQuery:
        response, err = s.handleMedicalQuery(req)
    case models.IntentGreeting:
        response, err = s.handleGreeting()
    default:
        response, err = s.handleUnknown(req)
    }
    
    if err != nil {
        return nil, err
    }
    
    // Save message to database
    message.BotResponse = response.Response
    message.IsAIResponse = (intent == models.IntentMedicalQuery || intent == models.IntentUnknown)
    
    // Save to database (implement this)
    // s.saveMessage(ctx, message)
    
    return response, nil
}

// All handler methods now return (*models.ChatResponse, error)

func (s *ChatbotService) handleEmergency() (*models.ChatResponse, error) {
    return &models.ChatResponse{
        Response: "🚨 EMERGENCY DETECTED! Please call 911 immediately or visit the nearest emergency room. " +
                 "For immediate assistance, call our emergency line: +1-234-567-8999",
        Intent: models.IntentEmergency,
        Actions: []models.Action{
            {
                Type:  "call",
                Label: "Call Emergency",
                Payload: map[string]interface{}{
                    "number": "911",
                },
            },
            {
                Type:  "call",
                Label: "Call Clinic Emergency",
                Payload: map[string]interface{}{
                    "number": "+1-234-567-8999",
                },
            },
        },
    }, nil // Added nil error return
}

// func (s *ChatbotService) handleAppointment(req models.ChatRequest) (*models.ChatResponse, error) {
//     message := strings.ToLower(req.Message)
    
//     // Handle different appointment-related queries
//     if strings.Contains(message, "cancel") {
//         return s.handleAppointmentCancellation(req)
//     } else if strings.Contains(message, "reschedule") {
//         return s.handleAppointmentReschedule(req)
//     } else if strings.Contains(message, "my appointments") || strings.Contains(message, "upcoming") {
//         return s.handleShowAppointments(req)
//     }
    
//     // Default: Show available slots
//     slots, err := s.appointmentSvc.GetAvailableSlots(
//         time.Now(), 
//         time.Now().AddDate(0, 0, 7), // Next 7 days
//     )
//     if err != nil {
//         return nil, fmt.Errorf("failed to get available slots: %w", err)
//     }
    
//     if len(slots) == 0 {
//         return &models.ChatResponse{
//             Response: "I'm sorry, but there are no available appointment slots in the next 7 days. " +
//                      "Would you like me to check for later dates or add you to our waiting list?",
//             Intent: models.IntentAppointment,
//             Actions: []models.Action{
//                 {
//                     Type:  "check_later_dates",
//                     Label: "Check Later Dates",
//                 },
//                 {
//                     Type:  "join_waitlist",
//                     Label: "Join Waiting List",
//                 },
//             },
//         }, nil
//     }
    
//     // Group slots by doctor
//     doctorSlots := make(map[string][]models.AppointmentSlot)
//     for _, slot := range slots {
//         key := fmt.Sprintf("%s (%s)", slot.DoctorName, slot.DoctorSpecialization)
//         doctorSlots[key] = append(doctorSlots[key], slot)
//     }
    
//     // Format response
//     responseText := "I can help you book an appointment. Here are available doctors and times:\n\n"
    
//     for doctor, doctorSlotList := range doctorSlots {
//         responseText += fmt.Sprintf("**%s**\n", doctor)
//         for i, slot := range doctorSlotList {
//             if i >= 3 { // Show only first 3 slots per doctor
//                 responseText += fmt.Sprintf("...and %d more slots\n", len(doctorSlotList)-3)
//                 break
//             }
//             responseText += fmt.Sprintf("• %s at %s ($%.2f)\n", 
//                 slot.StartTime.Format("Mon, Jan 2"),
//                 slot.StartTime.Format("3:04 PM"),
//                 slot.Fee,
//             )
//         }
//         responseText += "\n"
//     }
    
//     return &models.ChatResponse{
//         Response: responseText + "Which doctor would you like to see?",
//         Intent:   models.IntentAppointment,
//         Data: map[string]interface{}{
//             "slots": slots,
//             "grouped_slots": doctorSlots,
//         },
//         Actions: []models.Action{
//             {
//                 Type:  "select_doctor",
//                 Label: "Choose a Doctor",
//             },
//             {
//                 Type:  "filter_by_specialization",
//                 Label: "Filter by Specialization",
//             },
//             {
//                 Type:  "view_all_slots",
//                 Label: "View All Available Times",
//             },
//         },
//     }, nil
// }

func (s *ChatbotService) handleClinicInfo(message string) (*models.ChatResponse, error) {
    message = strings.ToLower(message)
    response := fmt.Sprintf("Here's information about %s:\n\n", s.clinicInfo["name"])
    
    // Build response based on what user is asking
    infoRequested := false
    
    if strings.Contains(message, "address") || strings.Contains(message, "location") || strings.Contains(message, "where") {
        response += fmt.Sprintf("📍 Address: %s\n", s.clinicInfo["address"])
        infoRequested = true
    }
    
    if strings.Contains(message, "phone") || strings.Contains(message, "contact") || strings.Contains(message, "call") {
        response += fmt.Sprintf("📞 Phone: %s\n", s.clinicInfo["phone"])
        infoRequested = true
    }
    
    if strings.Contains(message, "hours") || strings.Contains(message, "timing") || strings.Contains(message, "open") {
        response += fmt.Sprintf("🕐 Hours: %s\n", s.clinicInfo["hours"])
        infoRequested = true
    }
    
    if strings.Contains(message, "services") || strings.Contains(message, "specialization") || strings.Contains(message, "department") {
        response += fmt.Sprintf("🏥 Services: %s\n", s.clinicInfo["services"])
        infoRequested = true
    }
    
    // If no specific info requested, show all
    if !infoRequested {
        response = fmt.Sprintf(
            "Here's information about %s:\n\n"+
            "📍 Address: %s\n"+
            "📞 Phone: %s\n"+
            "🕐 Hours: %s\n"+
            "🏥 Services: %s\n\n"+
            "Is there anything specific you'd like to know?",
            s.clinicInfo["name"],
            s.clinicInfo["address"],
            s.clinicInfo["phone"],
            s.clinicInfo["hours"],
            s.clinicInfo["services"],
        )
    }
    
    return &models.ChatResponse{
        Response: response,
        Intent:   models.IntentClinicInfo,
        Actions: []models.Action{
            {
                Type:  "show_map",
                Label: "Show on Map",
                Payload: map[string]interface{}{
                    "address": s.clinicInfo["address"],
                },
            },
            {
                Type:  "call",
                Label: "Call Clinic",
                Payload: map[string]interface{}{
                    "number": s.clinicInfo["phone"],
                },
            },
            {
                Type:  "book_appointment",
                Label: "Book an Appointment",
            },
        },
    }, nil // Added nil error return
}

func (s *ChatbotService) handleMedicalQuery(req models.ChatRequest) (*models.ChatResponse, error) {
    // Add medical disclaimer
    prompt := fmt.Sprintf(
        "You are a medical assistant AI for a clinic. "+
        "IMPORTANT: Always remind users that this is not a replacement for professional medical advice. "+
        "User query: %s\n\n"+
        "Provide helpful general information while encouraging them to consult with a healthcare provider. "+
        "Keep the response concise and informative.",
        req.Message,
    )
    fmt.Println("prompt", prompt)
    aiResponse, err := s.aiService.GenerateResponse(prompt)

    if err != nil {
        fmt.Println("error", err)
        // Fallback response if AI fails
        return &models.ChatResponse{
            Response: "I apologize, but I'm having trouble processing your medical query right now. " +
                     "For medical concerns, it's always best to consult with our healthcare providers directly. " +
                     "Would you like to book an appointment?",
            Intent: models.IntentMedicalQuery,
            Actions: []models.Action{
                {
                    Type:  "book_consultation",
                    Label: "Book a Consultation",
                },
                {
                    Type:  "call_nurse",
                    Label: "Speak to a Nurse",
                },
            },
        }, nil
    }
    
    return &models.ChatResponse{
        Response: aiResponse + "\n\n⚠️ Note: This information is for educational purposes only. " +
                 "Please consult with our healthcare providers for personalized medical advice.",
        Intent: models.IntentMedicalQuery,
        Actions: []models.Action{
            {
                Type:  "book_consultation",
                Label: "Book a Consultation",
            },
            {
                Type:  "call_nurse",
                Label: "Speak to a Nurse",
            },
            {
                Type:  "view_doctors",
                Label: "View Our Doctors",
            },
        },
    }, nil
}

func (s *ChatbotService) handleGreeting() (*models.ChatResponse, error) {
    currentHour := time.Now().Hour()
    greeting := "Hello"
    
    if currentHour < 12 {
        greeting = "Good morning"
    } else if currentHour < 18 {
        greeting = "Good afternoon"
    } else {
        greeting = "Good evening"
    }
    
    return &models.ChatResponse{
        Response: fmt.Sprintf("%s! Welcome to %s. I'm here to help you with:\n\n", greeting, s.clinicInfo["name"]) +
                 "• 📅 Booking appointments\n" +
                 "• 🏥 Clinic information\n" +
                 "• 💊 General health queries\n" +
                 "• 🚨 Emergency assistance\n" +
                 "• 📋 Managing your appointments\n\n" +
                 "How can I assist you today?",
        Intent: models.IntentGreeting,
        Actions: []models.Action{
            {
                Type:  "quick_action",
                Label: "Book Appointment",
                Payload: map[string]interface{}{
                    "action": "book_appointment",
                },
            },
            {
                Type:  "quick_action",
                Label: "Clinic Hours",
                Payload: map[string]interface{}{
                    "action": "clinic_hours",
                },
            },
            {
                Type:  "quick_action",
                Label: "Emergency Help",
                Payload: map[string]interface{}{
                    "action": "emergency",
                },
            },
        },
    }, nil // Added nil error return
}

func (s *ChatbotService) handleUnknown(req models.ChatRequest) (*models.ChatResponse, error) {
    // Try to use AI for unknown queries
    return s.handleMedicalQuery(req)
}

// Additional handler methods

func (s *ChatbotService) handleAppointmentCancellation(req models.ChatRequest) (*models.ChatResponse, error) {
    // In a real implementation, you would extract appointment ID from context or ask user
    return &models.ChatResponse{
        Response: "I can help you cancel your appointment. Please provide your appointment ID or " +
                 "let me look up your upcoming appointments.",
        Intent: models.IntentAppointment,
        Actions: []models.Action{
            {
                Type:  "lookup_appointments",
                Label: "View My Appointments",
            },
            {
                Type:  "enter_appointment_id",
                Label: "Enter Appointment ID",
            },
        },
    }, nil
}

func (s *ChatbotService) handleAppointmentReschedule(req models.ChatRequest) (*models.ChatResponse, error) {
    return &models.ChatResponse{
        Response: "I can help you reschedule your appointment. First, let me find your current appointment. " +
                 "Please provide your appointment ID or shall I look up your appointments?",
        Intent: models.IntentAppointment,
        Actions: []models.Action{
            {
                Type:  "lookup_appointments",
                Label: "View My Appointments",
            },
            {
                Type:  "enter_appointment_id",
                Label: "Enter Appointment ID",
            },
        },
    }, nil
}

func (s *ChatbotService) handleShowAppointments(req models.ChatRequest) (*models.ChatResponse, error) {
    // if req.UserID == "" {
    //     return &models.ChatResponse{
    //         Response: "To view your appointments, I need to verify your identity. " +
    //                  "Please log in or provide your patient ID.",
    //         Intent: models.IntentAppointment,
    //         Actions: []models.Action{
    //             {
    //                 Type:  "login",
    //                 Label: "Log In",
    //             },
    //             {
    //                 Type:  "enter_patient_id",
    //                 Label: "Enter Patient ID",
    //             },
    //         },
    //     }, nil
    // }
    
    // // Get upcoming appointments
    // ctx := context.Background()
    // appointments, err := s.appointmentSvc.GetUpcomingAppointments(ctx, req.UserID)
    // if err != nil {
    //     return nil, fmt.Errorf("failed to get appointments: %w", err)
    // }
    
    // if len(appointments) == 0 {
    //     return &models.ChatResponse{
    //         Response: "You don't have any upcoming appointments. Would you like to book one?",
    //         Intent: models.IntentAppointment,
    //         Actions: []models.Action{
    //             {
    //                 Type:  "book_appointment",
    //                 Label: "Book New Appointment",
    //             },
    //         },
    //     }, nil
    // }
    
    // responseText := "Here are your upcoming appointments:\n\n"
    // for i, apt := range appointments {
    //     responseText += fmt.Sprintf("%d. **%s**\n", i+1, apt.StartTime.Format("Monday, Jan 2 at 3:04 PM"))
    //     responseText += fmt.Sprintf("   Doctor: Dr. %s\n", "Doctor Name") // You'd fetch actual doctor name
    //     responseText += fmt.Sprintf("   Status: %s\n", apt.Status)
    //     responseText += fmt.Sprintf("   Type: %s\n\n", apt.Type)
    // }
    
    // return &models.ChatResponse{
    //     Response: responseText,
    //     Intent: models.IntentAppointment,
    //     Data: map[string]interface{}{
    //         "appointments": appointments,
    //     },
    //     Actions: []models.Action{
    //         {
    //             Type:  "manage_appointment",
    //             Label: "Manage Appointments",
    //         },
    //         {
    //             Type:  "book_new",
    //             Label: "Book New Appointment",
    //         },
    //     },
    // }, nil
	return &models.ChatResponse{},nil
}

// Helper method to save messages (implement based on your database)
// func (s *ChatbotService) saveMessage(ctx context.Context, message *models.Message) error {
//     // Implementation depends on your database setup
//     return nil
// }
