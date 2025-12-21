package plugin

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Message represents a chat message
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// ChatRequest represents the incoming chat request
type ChatRequest struct {
	Messages         []Message              `json:"messages"`
	DashboardContext map[string]interface{} `json:"dashboardContext"`
}

// ChatResponse represents the chat response
type ChatResponse struct {
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Success   bool                   `json:"success"`
	Dashboard *DashboardInfo         `json:"dashboard,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// DashboardInfo contains created dashboard information
type DashboardInfo struct {
	URL   string `json:"url"`
	UID   string `json:"uid"`
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": "",
	})
}

// handleChat handles chat requests
func handleChat(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.DefaultLogger.Error("Failed to read request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Failed to read request body",
		})
		return
	}
	defer r.Body.Close()

	// Parse request
	var chatReq ChatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		log.DefaultLogger.Error("Failed to parse request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate settings
	if settings.OpenAIAPIKey == "" {
		log.DefaultLogger.Error("OpenAI API key not configured")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "OpenAI API key not configured. Please configure the plugin settings.",
		})
		return
	}

	// Process chat with OpenAI
	openaiService := NewOpenAIService(settings.OpenAIAPIKey, settings.AIEndpoint, settings.AIModel)
	aiResponse, err := openaiService.ProcessChat(chatReq.Messages)
	if err != nil {
		log.DefaultLogger.Error("OpenAI processing failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Failed to process chat: " + err.Error(),
		})
		return
	}

	// If AI wants to create a dashboard
	if aiResponse.Type == "function_call" && aiResponse.Function == "create_dashboard" {
		if settings.GrafanaURL == "" || settings.GrafanaToken == "" {
			json.NewEncoder(w).Encode(ChatResponse{
				Type:    "message",
				Message: "Para criar dashboards automaticamente, configure a URL e Token do Grafana nas configurações do plugin.",
				Success: true,
			})
			return
		}

		grafanaService := NewGrafanaService(settings.GrafanaURL, settings.GrafanaToken)
		dashboardResult, err := grafanaService.CreateDashboard(aiResponse.Data)
		if err != nil {
			log.DefaultLogger.Error("Dashboard creation failed", "error", err)
			json.NewEncoder(w).Encode(ChatResponse{
				Type:    "message",
				Message: "Desculpe, ocorreu um erro ao criar o dashboard: " + err.Error(),
				Success: true,
			})
			return
		}

		json.NewEncoder(w).Encode(ChatResponse{
			Type:    "dashboard_created",
			Message: aiResponse.Response,
			Success: true,
			Dashboard: &DashboardInfo{
				URL:   settings.GrafanaURL + "/d/" + dashboardResult.UID,
				UID:   dashboardResult.UID,
				ID:    dashboardResult.ID,
				Title: aiResponse.Data.Title,
			},
		})
		return
	}

	// Regular chat response
	json.NewEncoder(w).Encode(ChatResponse{
		Type:    "message",
		Message: aiResponse.Response,
		Success: true,
	})
}
