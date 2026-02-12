package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// SQLService handles SQL query execution through Grafana's datasource proxy
type SQLService struct {
	grafanaURL    string
	datasourceUID string
	tableName     string
	settings      AppSettings
	httpClient    *http.Client
}

// NewSQLService creates a new SQL service
func NewSQLService(grafanaURL, datasourceUID, tableName string, settings AppSettings) *SQLService {
	return &SQLService{
		grafanaURL:    grafanaURL,
		datasourceUID: datasourceUID,
		tableName:     tableName,
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

// SearchRecords searches for records matching the criteria
func (s *SQLService) SearchRecords(searchValues []string, searchByName bool, token string) ([]ServiceRecord, error) {
	// Build column list
	columns := []string{s.settings.PrimaryKeyColumn, s.settings.MaintenanceColumn}

	if s.settings.DisplayNameColumn != "" {
		columns = append(columns, s.settings.DisplayNameColumn)
	}
	if s.settings.SearchColumn != "" {
		columns = append(columns, s.settings.SearchColumn)
	}

	// Add additional columns
	if s.settings.AdditionalColumns != "" {
		for _, col := range strings.Split(s.settings.AdditionalColumns, ",") {
			col = strings.TrimSpace(col)
			if col != "" {
				columns = append(columns, col)
			}
		}
	}

	// Build WHERE clause for multiple values
	whereClause := "1=1"
	if len(searchValues) > 0 {
		conditions := []string{}
		searchColumn := s.settings.SearchColumn
		if searchByName && s.settings.DisplayNameColumn != "" {
			searchColumn = s.settings.DisplayNameColumn
		}

		if searchColumn == "" {
			searchColumn = s.settings.PrimaryKeyColumn
		}

		for _, val := range searchValues {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			// Escape single quotes for SQL injection prevention
			escapedVal := strings.ReplaceAll(val, "'", "''")

			if searchByName {
				// For name search, use LIKE
				conditions = append(conditions, fmt.Sprintf("%s LIKE '%%%s%%'", searchColumn, escapedVal))
			} else {
				// For ID search, use exact match
				conditions = append(conditions, fmt.Sprintf("%s = '%s'", searchColumn, escapedVal))
			}
		}

		if len(conditions) > 0 {
			whereClause = strings.Join(conditions, " OR ")
		}
	}

	// Build query - use TOP for MSSQL, LIMIT for others
	columnList := strings.Join(columns, ", ")
	query := fmt.Sprintf(`
		SELECT TOP 500 %s
		FROM %s
		WHERE %s
		ORDER BY %s
	`, columnList, s.tableName, whereClause, s.settings.PrimaryKeyColumn)

	log.DefaultLogger.Info("Executing search query", "query", query)

	records, err := s.executeQuery(query, token)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	return records, nil
}

// UpdateMaintenance updates the maintenance status of a record
func (s *SQLService) UpdateMaintenance(id interface{}, manutencao bool, token string) error {
	manutencaoValue := 0
	if manutencao {
		manutencaoValue = 1
	}

	// Format ID based on type
	idStr := fmt.Sprintf("%v", id)
	idStr = strings.ReplaceAll(idStr, "'", "''")

	query := fmt.Sprintf(`
		UPDATE %s
		SET %s = %d
		WHERE %s = '%s'
	`, s.tableName, s.settings.MaintenanceColumn, manutencaoValue, s.settings.PrimaryKeyColumn, idStr)

	log.DefaultLogger.Info("Executing update query", "query", query)

	_, err := s.executeQuery(query, token)
	if err != nil {
		return fmt.Errorf("failed to execute update query: %w", err)
	}

	return nil
}

// executeQuery executes a SQL query through Grafana's datasource proxy
func (s *SQLService) executeQuery(rawSQL string, token string) ([]ServiceRecord, error) {
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
		log.DefaultLogger.Error("Query failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result, ok := queryResp.Results["A"]; ok {
		if result.Error != "" {
			return nil, fmt.Errorf("query error: %s", result.Error)
		}
		return s.parseRecordsFromFrames(result.Frames)
	}

	return []ServiceRecord{}, nil
}

// parseRecordsFromFrames converts Grafana data frames to ServiceRecord slice
func (s *SQLService) parseRecordsFromFrames(frames []Frame) ([]ServiceRecord, error) {
	records := []ServiceRecord{}

	for _, frame := range frames {
		if len(frame.Schema.Fields) == 0 || len(frame.Data.Values) == 0 {
			continue
		}

		// Build field index map (case-insensitive)
		fieldIndex := make(map[string]int)
		for i, field := range frame.Schema.Fields {
			fieldIndex[strings.ToLower(field.Name)] = i
		}

		// Determine number of rows
		numRows := 0
		if len(frame.Data.Values) > 0 {
			numRows = len(frame.Data.Values[0])
		}

		// Parse each row
		for row := 0; row < numRows; row++ {
			record := ServiceRecord{
				AdditionalData: make(map[string]interface{}),
			}

			// Primary key
			pkCol := strings.ToLower(s.settings.PrimaryKeyColumn)
			if idx, ok := fieldIndex[pkCol]; ok && idx < len(frame.Data.Values) {
				record.ID = frame.Data.Values[idx][row]
			}

			// Maintenance status
			maintCol := strings.ToLower(s.settings.MaintenanceColumn)
			if idx, ok := fieldIndex[maintCol]; ok && idx < len(frame.Data.Values) {
				record.Manutencao = toBool(frame.Data.Values[idx][row])
			}

			// Display name
			if s.settings.DisplayNameColumn != "" {
				nameCol := strings.ToLower(s.settings.DisplayNameColumn)
				if idx, ok := fieldIndex[nameCol]; ok && idx < len(frame.Data.Values) {
					record.DisplayName = toString(frame.Data.Values[idx][row])
				}
			}

			// Search value
			if s.settings.SearchColumn != "" {
				searchCol := strings.ToLower(s.settings.SearchColumn)
				if idx, ok := fieldIndex[searchCol]; ok && idx < len(frame.Data.Values) {
					record.SearchValue = frame.Data.Values[idx][row]
				}
			}

			// Additional columns
			if s.settings.AdditionalColumns != "" {
				for _, col := range strings.Split(s.settings.AdditionalColumns, ",") {
					col = strings.TrimSpace(col)
					if col == "" {
						continue
					}
					colLower := strings.ToLower(col)
					if idx, ok := fieldIndex[colLower]; ok && idx < len(frame.Data.Values) {
						record.AdditionalData[col] = frame.Data.Values[idx][row]
					}
				}
			}

			records = append(records, record)
		}
	}

	return records, nil
}

// Helper functions for type conversion
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
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
