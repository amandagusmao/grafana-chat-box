package plugin

import "time"

// AppSettings contains the plugin settings from Grafana
type AppSettings struct {
	// Connection
	DatasourceUID string `json:"datasourceUid"`
	GrafanaURL    string `json:"grafanaUrl"`
	GrafanaToken  string `json:"grafanaToken"`

	// Query configuration
	SelectQuery       string `json:"selectQuery"`       // Custom SELECT query (can include JOINs)
	UpdateTable       string `json:"updateTable"`       // Table name for UPDATE operations
	IdColumn          string `json:"idColumn"`          // Column for search and UPDATE WHERE clause (e.g., "id_cadastro")
	NameColumn        string `json:"nameColumn"`        // Column that displays the name (e.g., "nome")
	MaintenanceColumn string `json:"maintenanceColumn"` // Column to update (0/1)

	// Audit configuration
	AuditTable string `json:"auditTable"` // Table name for audit logs (optional)

	// Access control
	AllowedOrgID string `json:"allowedOrgId"`
}

// DatasourceInfo represents a Grafana datasource
type DatasourceInfo struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
}

// ServiceRecord represents a record from the SQL table (dynamic)
type ServiceRecord struct {
	ID         interface{}            `json:"id"`
	Nome       string                 `json:"nome"`
	Manutencao bool                   `json:"manutencao"`
	Fields     map[string]interface{} `json:"fields,omitempty"` // All other fields from query
}

// SearchRequest represents the search request parameters
type SearchRequest struct {
	SearchValues []string `json:"searchValues"` // Multiple values (IDs or names)
	SearchByName bool     `json:"searchByName"` // true = search by name, false = search by ID
}

// SearchResponse represents the search response
type SearchResponse struct {
	Success bool            `json:"success"`
	Records []ServiceRecord `json:"records"`
	Error   string          `json:"error,omitempty"`
	Columns []string        `json:"columns,omitempty"` // Column names from query result
}

// UpdateRequest represents the update request parameters
type UpdateRequest struct {
	ID         interface{} `json:"id"`
	Manutencao bool        `json:"manutencao"`
	RecordName string      `json:"recordName"` // For audit logging
}

// UpdateResponse represents the update response
type UpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// PermissionResponse represents the permission check response
type PermissionResponse struct {
	HasPermission bool   `json:"hasPermission"`
	CurrentOrgID  int64  `json:"currentOrgId"`
	AllowedOrgID  int64  `json:"allowedOrgId"`
	UserLogin     string `json:"userLogin"`
	UserEmail     string `json:"userEmail"`
	Message       string `json:"message,omitempty"`
}

// ConfigResponse returns current plugin configuration to frontend
type ConfigResponse struct {
	Success           bool   `json:"success"`
	IdColumn          string `json:"idColumn"`
	NameColumn        string `json:"nameColumn"`
	MaintenanceColumn string `json:"maintenanceColumn"`
	HasAuditTable     bool   `json:"hasAuditTable"`
	Error             string `json:"error,omitempty"`
}

// DatasourcesResponse represents the datasources list response
type DatasourcesResponse struct {
	Datasources []DatasourceInfo `json:"datasources"`
}

// LoggedUser represents the currently logged in user
type LoggedUser struct {
	Login string
	Email string
	Role  string
	OrgID int64
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID            int64     `json:"id"`
	UserLogin     string    `json:"userLogin"`
	UserEmail     string    `json:"userEmail"`
	Action        string    `json:"action"`
	RecordID      string    `json:"recordId"`
	RecordName    string    `json:"recordName"`
	OldValue      bool      `json:"oldValue"`
	NewValue      bool      `json:"newValue"`
	Timestamp     time.Time `json:"timestamp"`
	TimestampStr  string    `json:"timestampStr"` // Formatted for display
}

// AuditRequest represents audit log query parameters
type AuditRequest struct {
	RecordID string `json:"recordId"` // Optional: filter by record ID
	Limit    int    `json:"limit"`    // Max records to return
	Offset   int    `json:"offset"`   // Pagination offset
}

// AuditResponse represents the audit log response
type AuditResponse struct {
	Success bool         `json:"success"`
	Entries []AuditEntry `json:"entries"`
	Total   int          `json:"total"`
	Error   string       `json:"error,omitempty"`
}
