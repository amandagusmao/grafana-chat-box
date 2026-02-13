package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// SQLService handles SQL query execution through Grafana's datasource proxy
type SQLService struct {
	grafanaURL    string
	datasourceUID string
	settings      AppSettings
	httpClient    *http.Client
}

// NewSQLService creates a new SQL service
func NewSQLService(grafanaURL, datasourceUID string, settings AppSettings) *SQLService {
	return &SQLService{
		grafanaURL:    grafanaURL,
		datasourceUID: datasourceUID,
		settings:      settings,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// QueryRequest represents a datasource query request
type QueryRequest struct {
	Queries []Query `json:"queries"`
	From    string  `json:"from"`
	To      string  `json:"to"`
}

// Query represents a single SQL query
type Query struct {
	RefID      string `json:"refId"`
	Datasource struct {
		UID  string `json:"uid"`
		Type string `json:"type"`
	} `json:"datasource"`
	RawSQL string `json:"rawSql"`
	Format string `json:"format"`
}

// QueryResponse represents the response from datasource query
type QueryResponse struct {
	Results map[string]QueryResult `json:"results"`
}

// QueryResult represents a single query result
type QueryResult struct {
	Frames []Frame `json:"frames"`
	Error  string  `json:"error,omitempty"`
}

// Frame represents a data frame
type Frame struct {
	Schema struct {
		Fields []FrameField `json:"fields"`
	} `json:"schema"`
	Data struct {
		Values [][]interface{} `json:"values"`
	} `json:"data"`
}

// FrameField represents a field in the frame
type FrameField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// extractTopClause extracts TOP N from a SQL query and returns the query without TOP and the TOP clause
func extractTopClause(query string) (string, string) {
	// Match TOP followed by a number (with optional parentheses)
	re := regexp.MustCompile(`(?i)\bTOP\s+(\d+|\(\d+\))\s*`)
	match := re.FindString(query)
	if match != "" {
		queryWithoutTop := re.ReplaceAllString(query, "")
		return queryWithoutTop, strings.TrimSpace(match)
	}
	return query, ""
}

// SearchRecords executes the custom SELECT query with search filters
func (s *SQLService) SearchRecords(searchValues []string, searchByName bool, token string) ([]ServiceRecord, []string, error) {
	// Get the base SELECT query from settings
	baseQuery := s.settings.SelectQuery
	if baseQuery == "" {
		return nil, nil, fmt.Errorf("SELECT query não configurada")
	}

	// Build WHERE clause for search
	whereClause := ""
	if len(searchValues) > 0 {
		conditions := []string{}

		// For name search, use the column name without alias (will search on subquery alias)
		// For ID search, use the full column reference
		var searchColumn string
		if searchByName {
			// Search by name - strip alias to search on computed column alias
			searchColumn = stripTableAlias(s.settings.NameColumn)
			if searchColumn == "" {
				searchColumn = "nome"
			}
		} else {
			// Search by ID uses the ID column with table alias (e.g., sc.id_cadastro)
			searchColumn = s.settings.IdColumn
			if searchColumn == "" {
				searchColumn = "id"
			}
		}

		for _, val := range searchValues {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			escapedVal := strings.ReplaceAll(val, "'", "''")

			if searchByName {
				conditions = append(conditions, fmt.Sprintf("%s LIKE '%%%s%%'", searchColumn, escapedVal))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = '%s'", searchColumn, escapedVal))
			}
		}

		if len(conditions) > 0 {
			whereClause = strings.Join(conditions, " OR ")
		}
	}

	var finalQuery string

	if searchByName && whereClause != "" {
		// For name search, wrap in subquery to allow searching on aliases/computed columns
		// Extract TOP clause to apply it on the outer query
		queryWithoutTop, topClause := extractTopClause(baseQuery)

		if topClause != "" {
			finalQuery = fmt.Sprintf("SELECT %s * FROM (%s) AS search_base WHERE %s",
				topClause, queryWithoutTop, whereClause)
		} else {
			finalQuery = fmt.Sprintf("SELECT TOP 500 * FROM (%s) AS search_base WHERE %s",
				queryWithoutTop, whereClause)
		}
	} else if strings.Contains(baseQuery, "{{WHERE}}") {
		// Query has placeholder
		if whereClause != "" {
			finalQuery = strings.Replace(baseQuery, "{{WHERE}}", "WHERE "+whereClause, 1)
		} else {
			finalQuery = strings.Replace(baseQuery, "{{WHERE}}", "", 1)
		}
	} else if strings.Contains(strings.ToUpper(baseQuery), "WHERE") {
		// Query already has WHERE, append with AND
		if whereClause != "" {
			finalQuery = baseQuery + " AND (" + whereClause + ")"
		} else {
			finalQuery = baseQuery
		}
	} else {
		// No WHERE in query, add it
		if whereClause != "" {
			finalQuery = baseQuery + " WHERE " + whereClause
		} else {
			finalQuery = baseQuery
		}
	}

	log.DefaultLogger.Info("Executing search query", "query", finalQuery)

	records, columns, err := s.executeSelectQuery(finalQuery, token)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	return records, columns, nil
}

// stripTableAlias removes table alias from column name (e.g., "sc.id" -> "id")
func stripTableAlias(column string) string {
	if idx := strings.LastIndex(column, "."); idx != -1 {
		return column[idx+1:]
	}
	return column
}

// GetCurrentMaintenanceStatus retrieves the current maintenance status of a record
func (s *SQLService) GetCurrentMaintenanceStatus(id interface{}, token string) (bool, error) {
	if s.settings.UpdateTable == "" {
		return false, fmt.Errorf("tabela não configurada")
	}

	idStr := fmt.Sprintf("%v", id)
	// SECURITY: Validate ID contains only safe characters (alphanumeric, dash, underscore)
	if !isValidID(idStr) {
		return false, fmt.Errorf("ID contém caracteres inválidos")
	}
	idStr = strings.ReplaceAll(idStr, "'", "''")

	idColumn := stripTableAlias(s.settings.IdColumn)
	maintColumn := stripTableAlias(s.settings.MaintenanceColumn)

	query := fmt.Sprintf(`
		SELECT %s FROM %s WHERE %s = '%s'
	`, maintColumn, s.settings.UpdateTable, idColumn, idStr)

	log.DefaultLogger.Info("Getting current maintenance status", "query", query)

	records, _, err := s.executeSelectQuery(query, token)
	if err != nil {
		return false, fmt.Errorf("failed to get current status: %w", err)
	}

	if len(records) == 0 {
		return false, fmt.Errorf("registro não encontrado")
	}

	return records[0].Manutencao, nil
}

// isValidID checks if the ID contains only safe characters
func isValidID(id string) bool {
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' || c == '_') {
			return false
		}
	}
	return len(id) > 0 && len(id) <= 100
}

// UpdateMaintenance updates the maintenance status of a record
func (s *SQLService) UpdateMaintenance(id interface{}, manutencao bool, token string) error {
	if s.settings.UpdateTable == "" {
		return fmt.Errorf("tabela de UPDATE não configurada")
	}

	manutencaoValue := 0
	if manutencao {
		manutencaoValue = 1
	}

	idStr := fmt.Sprintf("%v", id)

	// SECURITY: Validate ID contains only safe characters
	if !isValidID(idStr) {
		return fmt.Errorf("ID contém caracteres inválidos")
	}
	idStr = strings.ReplaceAll(idStr, "'", "''")

	// Strip table alias for UPDATE query (e.g., "sc.id_cadastro" -> "id_cadastro")
	idColumn := stripTableAlias(s.settings.IdColumn)
	maintColumn := stripTableAlias(s.settings.MaintenanceColumn)

	query := fmt.Sprintf(`
		UPDATE %s
		SET %s = %d
		WHERE %s = '%s'
	`, s.settings.UpdateTable, maintColumn, manutencaoValue, idColumn, idStr)

	log.DefaultLogger.Info("Executing update query", "query", query)

	_, _, err := s.executeSelectQuery(query, token)
	if err != nil {
		return fmt.Errorf("failed to execute update query: %w", err)
	}

	return nil
}

// LogAudit logs an audit entry to the audit table
// SECURITY: This function MUST succeed for any update to proceed
func (s *SQLService) LogAudit(user LoggedUser, action, recordID, recordName string, oldValue, newValue bool, token string) error {
	// SECURITY: Audit table is REQUIRED - no silent skipping allowed
	if s.settings.AuditTable == "" {
		return fmt.Errorf("tabela de auditoria não configurada - auditoria é obrigatória")
	}

	// SECURITY: User must be identified
	if user.Login == "" {
		return fmt.Errorf("usuário não identificado - auditoria requer identificação")
	}

	// Sanitize all inputs
	escapedLogin := strings.ReplaceAll(user.Login, "'", "''")
	escapedEmail := strings.ReplaceAll(user.Email, "'", "''")
	escapedRecordID := strings.ReplaceAll(recordID, "'", "''")
	escapedAction := strings.ReplaceAll(action, "'", "''")

	// Truncate and sanitize record name to prevent very long strings
	safeName := recordName
	if len(safeName) > 450 {
		safeName = safeName[:450]
	}
	escapedRecordName := strings.ReplaceAll(safeName, "'", "''")

	oldVal := 0
	if oldValue {
		oldVal = 1
	}
	newVal := 0
	if newValue {
		newVal = 1
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (user_login, user_email, action, record_id, record_name, old_value, new_value, timestamp)
		VALUES ('%s', '%s', '%s', '%s', '%s', %d, %d, GETDATE())
	`, s.settings.AuditTable, escapedLogin, escapedEmail, escapedAction, escapedRecordID, escapedRecordName, oldVal, newVal)

	log.DefaultLogger.Info("Logging audit entry", "user", user.Login, "action", action, "recordId", recordID)

	_, _, err := s.executeSelectQuery(query, token)
	if err != nil {
		log.DefaultLogger.Error("CRITICAL: Failed to log audit entry", "error", err)
		return fmt.Errorf("falha ao registrar auditoria: %w", err)
	}

	return nil
}

// GetAuditLogs retrieves audit logs from the audit table
func (s *SQLService) GetAuditLogs(recordID string, limit, offset int, token string) ([]AuditEntry, int, error) {
	if s.settings.AuditTable == "" {
		return nil, 0, fmt.Errorf("tabela de auditoria não configurada")
	}

	if limit <= 0 {
		limit = 100
	}

	// Build WHERE clause
	whereClause := ""
	if recordID != "" {
		escapedID := strings.ReplaceAll(recordID, "'", "''")
		whereClause = fmt.Sprintf("WHERE record_id = '%s'", escapedID)
	}

	// Get audit entries using OFFSET/FETCH (SQL Server 2012+)
	// Note: Cannot use TOP with OFFSET, must use FETCH NEXT instead
	query := fmt.Sprintf(`
		SELECT id, user_login, user_email, action, record_id, record_name, old_value, new_value, timestamp
		FROM %s
		%s
		ORDER BY timestamp DESC
		OFFSET %d ROWS FETCH NEXT %d ROWS ONLY
	`, s.settings.AuditTable, whereClause, offset, limit)

	log.DefaultLogger.Info("Fetching audit logs", "query", query)

	entries, err := s.executeAuditQuery(query, token)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch audit logs: %w", err)
	}

	return entries, len(entries), nil
}

// executeSelectQuery executes a SQL query and returns records
func (s *SQLService) executeSelectQuery(rawSQL string, token string) ([]ServiceRecord, []string, error) {
	reqPayload := QueryRequest{
		Queries: []Query{
			{
				RefID: "A",
				Datasource: struct {
					UID  string `json:"uid"`
					Type string `json:"type"`
				}{
					UID:  s.datasourceUID,
					Type: "mssql",
				},
				RawSQL: rawSQL,
				Format: "table",
			},
		},
		From: "now-1h",
		To:   "now",
	}

	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/ds/query", s.grafanaURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		log.DefaultLogger.Error("Query failed", "status", resp.StatusCode, "body", string(body))
		return nil, nil, fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result, ok := queryResp.Results["A"]; ok {
		if result.Error != "" {
			return nil, nil, fmt.Errorf("query error: %s", result.Error)
		}
		return s.parseRecordsFromFrames(result.Frames)
	}

	return []ServiceRecord{}, []string{}, nil
}

// executeAuditQuery executes a query and returns audit entries
func (s *SQLService) executeAuditQuery(rawSQL string, token string) ([]AuditEntry, error) {
	reqPayload := QueryRequest{
		Queries: []Query{
			{
				RefID: "A",
				Datasource: struct {
					UID  string `json:"uid"`
					Type string `json:"type"`
				}{
					UID:  s.datasourceUID,
					Type: "mssql",
				},
				RawSQL: rawSQL,
				Format: "table",
			},
		},
		From: "now-1h",
		To:   "now",
	}

	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/ds/query", s.grafanaURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		log.DefaultLogger.Error("Audit query failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	entries := []AuditEntry{}
	if result, ok := queryResp.Results["A"]; ok {
		if result.Error != "" {
			return nil, fmt.Errorf("query error: %s", result.Error)
		}
		entries = s.parseAuditFromFrames(result.Frames)
	}

	return entries, nil
}

// parseRecordsFromFrames converts Grafana data frames to ServiceRecord slice
func (s *SQLService) parseRecordsFromFrames(frames []Frame) ([]ServiceRecord, []string, error) {
	records := []ServiceRecord{}
	columns := []string{}

	for _, frame := range frames {
		if len(frame.Schema.Fields) == 0 || len(frame.Data.Values) == 0 {
			continue
		}

		// Build field index map (case-insensitive)
		fieldIndex := make(map[string]int)
		for i, field := range frame.Schema.Fields {
			fieldIndex[strings.ToLower(field.Name)] = i
			columns = append(columns, field.Name)
		}

		// Determine number of rows
		numRows := 0
		if len(frame.Data.Values) > 0 {
			numRows = len(frame.Data.Values[0])
		}

		// Parse each row
		for row := 0; row < numRows; row++ {
			record := ServiceRecord{
				Fields: make(map[string]interface{}),
			}

			// ID column - strip table alias for lookup (e.g., "sc.id_cadastro" -> "id_cadastro")
			idCol := strings.ToLower(stripTableAlias(s.settings.IdColumn))
			if idCol == "" {
				idCol = "id"
			}
			if idx, ok := fieldIndex[idCol]; ok && idx < len(frame.Data.Values) {
				record.ID = frame.Data.Values[idx][row]
			}

			// Maintenance status - strip table alias for lookup
			maintCol := strings.ToLower(stripTableAlias(s.settings.MaintenanceColumn))
			if maintCol == "" {
				maintCol = "manutencao"
			}
			if idx, ok := fieldIndex[maintCol]; ok && idx < len(frame.Data.Values) {
				record.Manutencao = toBool(frame.Data.Values[idx][row])
			}

			// Name column - strip table alias for lookup
			nameCol := strings.ToLower(stripTableAlias(s.settings.NameColumn))
			if nameCol == "" {
				nameCol = "nome"
			}
			if idx, ok := fieldIndex[nameCol]; ok && idx < len(frame.Data.Values) {
				record.Nome = toString(frame.Data.Values[idx][row])
			}

			// All other fields
			for i, field := range frame.Schema.Fields {
				if i < len(frame.Data.Values) {
					record.Fields[field.Name] = frame.Data.Values[i][row]
				}
			}

			records = append(records, record)
		}
	}

	return records, columns, nil
}

// parseAuditFromFrames converts Grafana data frames to AuditEntry slice
func (s *SQLService) parseAuditFromFrames(frames []Frame) []AuditEntry {
	entries := []AuditEntry{}

	for _, frame := range frames {
		if len(frame.Schema.Fields) == 0 || len(frame.Data.Values) == 0 {
			continue
		}

		fieldIndex := make(map[string]int)
		for i, field := range frame.Schema.Fields {
			fieldIndex[strings.ToLower(field.Name)] = i
		}

		numRows := 0
		if len(frame.Data.Values) > 0 {
			numRows = len(frame.Data.Values[0])
		}

		for row := 0; row < numRows; row++ {
			entry := AuditEntry{}

			if idx, ok := fieldIndex["id"]; ok && idx < len(frame.Data.Values) {
				entry.ID = toInt64(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["user_login"]; ok && idx < len(frame.Data.Values) {
				entry.UserLogin = toString(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["user_email"]; ok && idx < len(frame.Data.Values) {
				entry.UserEmail = toString(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["action"]; ok && idx < len(frame.Data.Values) {
				entry.Action = toString(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["record_id"]; ok && idx < len(frame.Data.Values) {
				entry.RecordID = toString(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["record_name"]; ok && idx < len(frame.Data.Values) {
				entry.RecordName = toString(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["old_value"]; ok && idx < len(frame.Data.Values) {
				entry.OldValue = toBool(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["new_value"]; ok && idx < len(frame.Data.Values) {
				entry.NewValue = toBool(frame.Data.Values[idx][row])
			}
			if idx, ok := fieldIndex["timestamp"]; ok && idx < len(frame.Data.Values) {
				entry.Timestamp = toTime(frame.Data.Values[idx][row])
				entry.TimestampStr = entry.Timestamp.Format("02/01/2006 15:04:05")
			}

			entries = append(entries, entry)
		}
	}

	return entries
}

// Helper functions for type conversion
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
		return val != 0
	case string:
		return val == "true" || val == "1"
	default:
		return false
	}
}

func toTime(v interface{}) time.Time {
	switch val := v.(type) {
	case time.Time:
		return val
	case string:
		t, err := time.Parse(time.RFC3339, val)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05Z", val)
			if err != nil {
				return time.Time{}
			}
		}
		return t
	case float64:
		// Unix timestamp in milliseconds
		return time.Unix(int64(val)/1000, 0)
	case int64:
		return time.Unix(val/1000, 0)
	default:
		return time.Time{}
	}
}
