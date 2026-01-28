package plugin

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

// App is the main plugin app instance
type App struct {
	backend.CallResourceHandler
}

// AppSettings contains the plugin settings from Grafana
type AppSettings struct {
	Identificador string `json:"identificador"`
	Senha         string `json:"senha"`
	GrafanaURL    string `json:"grafanaUrl"`
	GrafanaToken  string `json:"grafanaToken"`
	AIEndpoint    string `json:"aiEndpoint"`
	AIModel       string `json:"aiModel"`
	// Default datasources configuration
	DefaultPrometheusUID string `json:"defaultPrometheusUid"`
	DefaultLokiUID       string `json:"defaultLokiUid"`
	DefaultTempoUID      string `json:"defaultTempoUid"`
}

// NewApp creates a new App instance
func NewApp(ctx context.Context, settings backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	log.DefaultLogger.Info("Creating new App instance")

	var appSettings AppSettings
	if settings.JSONData != nil && len(settings.JSONData) > 0 {
		if err := json.Unmarshal(settings.JSONData, &appSettings); err != nil {
			log.DefaultLogger.Warn("Failed to parse app settings", "error", err)
		}
	}

	// Get secure settings (credentials)
	if identificador, exists := settings.DecryptedSecureJSONData["identificador"]; exists {
		appSettings.Identificador = identificador
	}
	if senha, exists := settings.DecryptedSecureJSONData["senha"]; exists {
		appSettings.Senha = senha
	}
	if token, exists := settings.DecryptedSecureJSONData["grafanaToken"]; exists {
		appSettings.GrafanaToken = token
	}

	app := &App{}

	// Create AuthService once (singleton for token caching)
	var authService *AuthService
	if appSettings.AIEndpoint != "" && appSettings.Identificador != "" && appSettings.Senha != "" {
		authService = NewAuthService(appSettings.AIEndpoint, appSettings.Identificador, appSettings.Senha)
		log.DefaultLogger.Info("AuthService initialized")
	}

	// Create HTTP router for resource calls
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, appSettings, authService)
	})
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/datasources", func(w http.ResponseWriter, r *http.Request) {
		handleListDatasources(w, r, appSettings)
	})

	app.CallResourceHandler = httpadapter.New(mux)

	return app, nil
}

// CheckHealth handles health check requests
func (a *App) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Plugin is running",
	}, nil
}

// Dispose cleans up resources
func (a *App) Dispose() {
	log.DefaultLogger.Info("Disposing App instance")
}
