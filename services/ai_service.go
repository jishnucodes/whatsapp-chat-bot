package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type AIService struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
}

func NewAIService() *AIService {
	return &AIService{
		apiKey: os.Getenv("GOOGLE_API_KEY"),
		apiURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *AIService) GenerateResponse(prompt string) (string, error) {
	endpoint := fmt.Sprintf("%s?key=%s", s.apiURL, s.apiKey)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 500,
		},
		"safetySettings": []map[string]interface{}{
			{
				"category":  "HARM_CATEGORY_HARASSMENT",
				"threshold": "BLOCK_ONLY_HIGH",
			},
			{
				"category":  "HARM_CATEGORY_HATE_SPEECH",
				"threshold": "BLOCK_ONLY_HIGH",
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	fmt.Println("json data", string(jsonData))

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API error: %s", string(body))
	}

	// Parse response (implement based on actual response structure)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Extract text from response
	candidates := result["candidates"].([]interface{})
	if len(candidates) > 0 {
		candidate := candidates[0].(map[string]interface{})
		content := candidate["content"].(map[string]interface{})
		parts := content["parts"].([]interface{})
		if len(parts) > 0 {
			part := parts[0].(map[string]interface{})
			return part["text"].(string), nil
		}
	}

	return "", fmt.Errorf("no response generated")
}
