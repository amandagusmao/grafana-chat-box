package plugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Maintenance Manager is running",
	})
}

// handleGetConfig returns the current table configuration to frontend
func handleGetConfig(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Parse additional columns
	additionalCols := []string{}
	if settings.AdditionalColumns != "" {
		for _, col := range strings.Split(settings.AdditionalColumns, ",") {
			col = strings.TrimSpace(col)
			if col != "" {
				additionalCols = append(additionalCols, col)
			}
		}
	}

	json.NewEncoder(w).Encode(ConfigResponse{
		Success: true,
		TableConfig: &TableConfig{
			PrimaryKeyColumn:  settings.PrimaryKeyColumn,
			MaintenanceColumn: settings.MaintenanceColumn,
			SearchColumn:      settings.SearchColumn,
			DisplayNameColumn: settings.DisplayNameColumn,
			AdditionalColumns: additionalCols,
		},
	})
}

// getLoggedUser extracts the logged user from request headers
func getLoggedUser(r *http.Request) *LoggedUser {
	user := &LoggedUser{}

	// Try X-Grafana-Id header (JWT) first
	grafanaID := r.Header.Get("X-Grafana-Id")
	if grafanaID != "" {
		parts := strings.Split(grafanaID, ".")
		if len(parts) >= 2 {
			payload := parts[1]
			switch len(payload) % 4 {
			case 2:
				payload += "=="
			case 3:
				payload += "="
			}

			decoded, err := base64.URLEncoding.DecodeString(payload)
			if err != nil {
				decoded, err = base64.StdEncoding.DecodeString(payload)
			}

			if err == nil {
				var claims struct {
					Email    string `json:"email"`
					Username string `json:"username"`
					Login    string `json:"login"`
					Sub      string `json:"sub"`
					Role     string `json:"role"`
					OrgID    int64  `json:"orgId"`
				}
				if err := json.Unmarshal(decoded, &claims); err == nil {
					user.Email = claims.Email
					user.Role = claims.Role
					user.OrgID = claims.OrgID
					if claims.Username != "" {
						user.Login = claims.Username
					} else if claims.Login != "" {
						user.Login = claims.Login
					} else if claims.Sub != "" {
						user.Login = claims.Sub
					}
				}
			}
		}
	}

	if user.Login == "" {
		if login := r.Header.Get("X-Grafana-User"); login != "" {
			user.Login = login
		}
	}

	if user.OrgID == 0 {
		if orgIDStr := r.Header.Get("X-Grafana-Org-Id"); orgIDStr != "" {
			if orgID, err := strconv.ParseInt(orgIDStr, 10, 64); err == nil {
				user.OrgID = orgID
			}
		}
	}

	return user
}

// handleCheckPermission checks if the current user has permission to modify records
func handleCheckPermission(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(PermissionResponse{
			HasPermission: false,
			Message:       "Method not allowed",
		})
		return
	}

	loggedUser := getLoggedUser(r)

	if settings.AllowedOrgID == "" {
		json.NewEncoder(w).Encode(PermissionResponse{
			HasPermission: true,
			CurrentOrgID:  loggedUser.OrgID,
			AllowedOrgID:  0,
			UserLogin:     loggedUser.Login,
		})
		return
	}

	allowedOrgID, err := strconv.ParseInt(settings.AllowedOrgID, 10, 64)
	if err != nil {
		json.NewEncoder(w).Encode(PermissionResponse{
			HasPermission: false,
			CurrentOrgID:  loggedUser.OrgID,
			AllowedOrgID:  0,
			UserLogin:     loggedUser.Login,
			Message:       "Configuração de organização inválida.",
		})
		return
	}

	hasPermission := loggedUser.OrgID == allowedOrgID

	response := PermissionResponse{
		HasPermission: hasPermission,
		CurrentOrgID:  loggedUser.OrgID,
		AllowedOrgID:  allowedOrgID,
		UserLogin:     loggedUser.Login,
	}

	if !hasPermission {
		response.Message = fmt.Sprintf("Você pertence à organização %d, mas apenas usuários da organização %d podem fazer alterações.",
			loggedUser.OrgID, allowedOrgID)
	}

	json.NewEncoder(w).Encode(response)
}

// getGrafanaURL extracts Grafana base URL from request
func getGrafanaURL(r *http.Request) string {
	origin := r.Header.Get("Origin")
	if origin != "" {
		return origin
	}

	referer := r.Header.Get("Referer")
	if referer != "" {
		if idx := strings.Index(referer, "/a/"); idx > 0 {
			return referer[:idx]
		}
		if idx := strings.Index(referer, "/d/"); idx > 0 {
			return referer[:idx]
		}
	}

	return "http://localhost:3000"
}

// handleSearch searches for records in the SQL table
func handleSearch(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Validate settings
	if settings.DatasourceUID == "" {
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   "Datasource não configurado. Acesse a página de configuração.",
		})
		return
	}

	if settings.TableName == "" {
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   "Nome da tabela não configurado. Acesse a página de configuração.",
		})
		return
	}

	if settings.GrafanaToken == "" {
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   "Token do Grafana não configurado. Acesse a página de configuração.",
		})
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   "Erro ao ler requisição.",
		})
		return
	}
	defer r.Body.Close()

	var searchReq SearchRequest
	if err := json.Unmarshal(body, &searchReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   "Formato de requisição inválido.",
		})
		return
	}

	log.DefaultLogger.Info("Search request",
		"values", searchReq.SearchValues,
		"searchByName", searchReq.SearchByName,
		"datasource", settings.DatasourceUID,
		"table", settings.TableName)

	grafanaURL := settings.GrafanaURL
	if grafanaURL == "" {
		grafanaURL = getGrafanaURL(r)
	}

	sqlService := NewSQLService(grafanaURL, settings.DatasourceUID, settings.TableName, settings)
	records, err := sqlService.SearchRecords(searchReq.SearchValues, searchReq.SearchByName, settings.GrafanaToken)
	if err != nil {
		log.DefaultLogger.Error("Search failed", "error", err)
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro ao buscar registros: %v", err),
		})
		return
	}

	// Parse additional columns for config response
	additionalCols := []string{}
	if settings.AdditionalColumns != "" {
		for _, col := range strings.Split(settings.AdditionalColumns, ",") {
			col = strings.TrimSpace(col)
			if col != "" {
				additionalCols = append(additionalCols, col)
			}
		}
	}

	json.NewEncoder(w).Encode(SearchResponse{
		Success: true,
		Records: records,
		Config: &TableConfig{
			PrimaryKeyColumn:  settings.PrimaryKeyColumn,
			MaintenanceColumn: settings.MaintenanceColumn,
			SearchColumn:      settings.SearchColumn,
			DisplayNameColumn: settings.DisplayNameColumn,
			AdditionalColumns: additionalCols,
		},
	})
}

// handleUpdate updates the maintenance status of a record
func handleUpdate(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Check permission first
	loggedUser := getLoggedUser(r)

	if settings.AllowedOrgID != "" {
		allowedOrgID, err := strconv.ParseInt(settings.AllowedOrgID, 10, 64)
		if err == nil && loggedUser.OrgID != allowedOrgID {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(UpdateResponse{
				Success: false,
				Error:   "Você não tem permissão para alterar registros.",
			})
			return
		}
	}

	// Validate settings
	if settings.DatasourceUID == "" {
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Datasource não configurado.",
		})
		return
	}

	if settings.TableName == "" {
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Nome da tabela não configurado.",
		})
		return
	}

	if settings.GrafanaToken == "" {
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Token do Grafana não configurado.",
		})
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Erro ao ler requisição.",
		})
		return
	}
	defer r.Body.Close()

	var updateReq UpdateRequest
	if err := json.Unmarshal(body, &updateReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Formato de requisição inválido.",
		})
		return
	}

	log.DefaultLogger.Info("Update request",
		"id", updateReq.ID,
		"manutencao", updateReq.Manutencao,
		"user", loggedUser.Login)

	grafanaURL := settings.GrafanaURL
	if grafanaURL == "" {
		grafanaURL = getGrafanaURL(r)
	}

	sqlService := NewSQLService(grafanaURL, settings.DatasourceUID, settings.TableName, settings)
	err = sqlService.UpdateMaintenance(updateReq.ID, updateReq.Manutencao, settings.GrafanaToken)
	if err != nil {
		log.DefaultLogger.Error("Update failed", "error", err)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro ao atualizar registro: %v", err),
		})
		return
	}

	statusText := "Normal"
	if updateReq.Manutencao {
		statusText = "Em Manutenção"
	}

	json.NewEncoder(w).Encode(UpdateResponse{
		Success: true,
		Message: fmt.Sprintf("Status alterado para '%s' com sucesso.", statusText),
	})
}
