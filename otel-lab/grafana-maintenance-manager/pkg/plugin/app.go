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

// NewApp creates a new App instance
func NewApp(ctx context.Context, settings backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	log.DefaultLogger.Info("Creating new Maintenance Manager App instance")

	var appSettings AppSettings
	if settings.JSONData != nil && len(settings.JSONData) > 0 {
		if err := json.Unmarshal(settings.JSONData, &appSettings); err != nil {
			log.DefaultLogger.Warn("Failed to parse app settings", "error", err)
		}
	}

	// Get secure settings (Grafana token)
	if token, exists := settings.DecryptedSecureJSONData["grafanaToken"]; exists {
		appSettings.GrafanaToken = token
	}

	// Set defaults for column names if not configured
	if appSettings.IdColumn == "" {
		appSettings.IdColumn = "id"
	}
	if appSettings.NameColumn == "" {
		appSettings.NameColumn = "nome"
	}
	if appSettings.MaintenanceColumn == "" {
		appSettings.MaintenanceColumn = "manutencao"
	}

	log.DefaultLogger.Info("App settings loaded",
		"datasourceUid", appSettings.DatasourceUID,
		"updateTable", appSettings.UpdateTable,
		"auditTable", appSettings.AuditTable,
		"idColumn", appSettings.IdColumn,
		"nameColumn", appSettings.NameColumn,
		"maintenanceColumn", appSettings.MaintenanceColumn,
		"hasSelectQuery", appSettings.SelectQuery != "",
		"hasToken", appSettings.GrafanaToken != "")

	app := &App{}

	// Create HTTP router for resource calls
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		handleGetConfig(w, r, appSettings)
	})
	mux.HandleFunc("/check-permission", func(w http.ResponseWriter, r *http.Request) {
		handleCheckPermission(w, r, appSettings)
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		handleSearch(w, r, appSettings)
	})
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		handleUpdate(w, r, appSettings)
	})
	mux.HandleFunc("/audit", func(w http.ResponseWriter, r *http.Request) {
		handleAudit(w, r, appSettings)
	})
	mux.HandleFunc("/audit/export", func(w http.ResponseWriter, r *http.Request) {
		handleAuditExport(w, r, appSettings)
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
	log.DefaultLogger.Info("Disposing Maintenance Manager App instance")
}
