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

// handleGetConfig returns the current configuration to frontend
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

	json.NewEncoder(w).Encode(ConfigResponse{
		Success:           true,
		IdColumn:          settings.IdColumn,
		NameColumn:        settings.NameColumn,
		MaintenanceColumn: settings.MaintenanceColumn,
		HasAuditTable:     settings.AuditTable != "",
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
			UserEmail:     loggedUser.Email,
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
			UserEmail:     loggedUser.Email,
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
		UserEmail:     loggedUser.Email,
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

// handleSearch searches for records using the custom SELECT query
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

	if settings.SelectQuery == "" {
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   "Query SELECT não configurada. Acesse a página de configuração.",
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
		"datasource", settings.DatasourceUID)

	grafanaURL := settings.GrafanaURL
	if grafanaURL == "" {
		grafanaURL = getGrafanaURL(r)
	}

	sqlService := NewSQLService(grafanaURL, settings.DatasourceUID, settings)
	records, columns, err := sqlService.SearchRecords(searchReq.SearchValues, searchReq.SearchByName, settings.GrafanaToken)
	if err != nil {
		log.DefaultLogger.Error("Search failed", "error", err)
		json.NewEncoder(w).Encode(SearchResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro ao buscar registros: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(SearchResponse{
		Success: true,
		Records: records,
		Columns: columns,
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

	// Validate user identification for audit
	if loggedUser.Login == "" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Não foi possível identificar o usuário. Faça login novamente.",
		})
		return
	}

	// Validate settings
	if settings.DatasourceUID == "" {
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Datasource não configurado.",
		})
		return
	}

	if settings.UpdateTable == "" {
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Tabela para UPDATE não configurada.",
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

	// SECURITY: Audit table MUST be configured - updates without audit are not allowed
	if settings.AuditTable == "" {
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "Tabela de auditoria não configurada. Todas as alterações devem ser auditadas.",
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

	// SECURITY: Validate that ID is provided and not empty
	if updateReq.ID == nil || fmt.Sprintf("%v", updateReq.ID) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   "ID do registro é obrigatório.",
		})
		return
	}

	log.DefaultLogger.Info("Update request",
		"id", updateReq.ID,
		"manutencao", updateReq.Manutencao,
		"user", loggedUser.Login,
		"email", loggedUser.Email)

	grafanaURL := settings.GrafanaURL
	if grafanaURL == "" {
		grafanaURL = getGrafanaURL(r)
	}

	sqlService := NewSQLService(grafanaURL, settings.DatasourceUID, settings)

	// SECURITY: Get the ACTUAL current value before updating (for accurate audit)
	oldValue, err := sqlService.GetCurrentMaintenanceStatus(updateReq.ID, settings.GrafanaToken)
	if err != nil {
		log.DefaultLogger.Error("Failed to get current status", "error", err)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro ao verificar status atual: %v", err),
		})
		return
	}

	// Check if value is actually changing
	if oldValue == updateReq.Manutencao {
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: true,
			Message: "O registro já está com o status solicitado.",
		})
		return
	}

	// SECURITY: Log audit FIRST, before the update
	// This ensures we have a record even if update fails partially
	action := "Colocou em Manutenção"
	if !updateReq.Manutencao {
		action = "Removeu da Manutenção"
	}

	recordID := fmt.Sprintf("%v", updateReq.ID)
	err = sqlService.LogAudit(*loggedUser, action, recordID, updateReq.RecordName, oldValue, updateReq.Manutencao, settings.GrafanaToken)
	if err != nil {
		log.DefaultLogger.Error("Failed to log audit - UPDATE BLOCKED", "error", err)
		json.NewEncoder(w).Encode(UpdateResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro ao registrar auditoria. Alteração bloqueada: %v", err),
		})
		return
	}

	// Execute update (only after audit is successfully logged)
	err = sqlService.UpdateMaintenance(updateReq.ID, updateReq.Manutencao, settings.GrafanaToken)
	if err != nil {
		log.DefaultLogger.Error("Update failed after audit logged", "error", err)
		// Note: Audit was already logged, so we have a record of the attempted change
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

// handleAudit retrieves audit logs
func handleAudit(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(AuditResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	if settings.AuditTable == "" {
		json.NewEncoder(w).Encode(AuditResponse{
			Success: false,
			Error:   "Tabela de auditoria não configurada.",
		})
		return
	}

	if settings.GrafanaToken == "" {
		json.NewEncoder(w).Encode(AuditResponse{
			Success: false,
			Error:   "Token do Grafana não configurado.",
		})
		return
	}

	// Parse request parameters
	var auditReq AuditRequest
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			json.Unmarshal(body, &auditReq)
		}
		defer r.Body.Close()
	} else {
		// GET parameters
		auditReq.RecordID = r.URL.Query().Get("recordId")
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			auditReq.Limit, _ = strconv.Atoi(limitStr)
		}
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			auditReq.Offset, _ = strconv.Atoi(offsetStr)
		}
	}

	if auditReq.Limit <= 0 {
		auditReq.Limit = 100
	}

	grafanaURL := settings.GrafanaURL
	if grafanaURL == "" {
		grafanaURL = getGrafanaURL(r)
	}

	sqlService := NewSQLService(grafanaURL, settings.DatasourceUID, settings)
	entries, total, err := sqlService.GetAuditLogs(auditReq.RecordID, auditReq.Limit, auditReq.Offset, settings.GrafanaToken)
	if err != nil {
		log.DefaultLogger.Error("Audit query failed", "error", err)
		json.NewEncoder(w).Encode(AuditResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro ao buscar auditoria: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(AuditResponse{
		Success: true,
		Entries: entries,
		Total:   total,
	})
}

// sanitizeCSVField prevents CSV formula injection by escaping dangerous characters
// Fields starting with =, +, -, @, tab, or carriage return are prefixed with a single quote
func sanitizeCSVField(field string) string {
	if len(field) == 0 {
		return field
	}
	// Check for dangerous first characters that could be interpreted as formulas
	firstChar := field[0]
	if firstChar == '=' || firstChar == '+' || firstChar == '-' || firstChar == '@' || firstChar == '\t' || firstChar == '\r' {
		return "'" + field
	}
	// Also escape semicolons and quotes within the field
	field = strings.ReplaceAll(field, "\"", "\"\"")
	if strings.ContainsAny(field, ";\n\"") {
		return "\"" + field + "\""
	}
	return field
}

// handleAuditExport exports audit logs as CSV
func handleAuditExport(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if settings.AuditTable == "" || settings.GrafanaToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Configuração incompleta"))
		return
	}

	recordID := r.URL.Query().Get("recordId")

	grafanaURL := settings.GrafanaURL
	if grafanaURL == "" {
		grafanaURL = getGrafanaURL(r)
	}

	sqlService := NewSQLService(grafanaURL, settings.DatasourceUID, settings)
	entries, _, err := sqlService.GetAuditLogs(recordID, 10000, 0, settings.GrafanaToken)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Erro ao buscar auditoria"))
		return
	}

	// Generate CSV
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=auditoria_manutencao.csv")

	// BOM for Excel UTF-8 compatibility
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	// CSV Header
	w.Write([]byte("Data/Hora;Usuario;Email;Acao;ID Registro;Nome Registro;Valor Anterior;Novo Valor\n"))

	// CSV Data - all fields sanitized against formula injection
	for _, entry := range entries {
		oldVal := "Normal"
		if entry.OldValue {
			oldVal = "Em Manutencao"
		}
		newVal := "Normal"
		if entry.NewValue {
			newVal = "Em Manutencao"
		}

		line := fmt.Sprintf("%s;%s;%s;%s;%s;%s;%s;%s\n",
			sanitizeCSVField(entry.TimestampStr),
			sanitizeCSVField(entry.UserLogin),
			sanitizeCSVField(entry.UserEmail),
			sanitizeCSVField(entry.Action),
			sanitizeCSVField(entry.RecordID),
			sanitizeCSVField(entry.RecordName),
			oldVal,
			newVal,
		)
		w.Write([]byte(line))
	}
}
