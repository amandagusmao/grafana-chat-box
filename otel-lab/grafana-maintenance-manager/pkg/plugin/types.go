package plugin

// AppSettings contains the plugin settings from Grafana
type AppSettings struct {
	// Connection
	DatasourceUID string `json:"datasourceUid"`
	GrafanaURL    string `json:"grafanaUrl"`
	GrafanaToken  string `json:"grafanaToken"`

	// Table configuration
	TableName           string `json:"tableName"`
	PrimaryKeyColumn    string `json:"primaryKeyColumn"`    // ex: "id"
	MaintenanceColumn   string `json:"maintenanceColumn"`   // ex: "manutencao"
	SearchColumn        string `json:"searchColumn"`        // ex: "id_cadastro"
	DisplayNameColumn   string `json:"displayNameColumn"`   // ex: "nome"
	AdditionalColumns   string `json:"additionalColumns"`   // ex: "id_servico,ativo" (comma separated)

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
	ID              interface{}            `json:"id"`
	DisplayName     string                 `json:"displayName"`
	SearchValue     interface{}            `json:"searchValue"`
	Manutencao      bool                   `json:"manutencao"`
	AdditionalData  map[string]interface{} `json:"additionalData,omitempty"`
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
	Config  *TableConfig    `json:"config,omitempty"`
}

// TableConfig returns the configured column names for frontend display
type TableConfig struct {
	PrimaryKeyColumn  string   `json:"primaryKeyColumn"`
	MaintenanceColumn string   `json:"maintenanceColumn"`
	SearchColumn      string   `json:"searchColumn"`
	DisplayNameColumn string   `json:"displayNameColumn"`
	AdditionalColumns []string `json:"additionalColumns"`
}

// UpdateRequest represents the update request parameters
type UpdateRequest struct {
	ID         interface{} `json:"id"`
	Manutencao bool        `json:"manutencao"`
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
	Message       string `json:"message,omitempty"`
}

// ConfigResponse returns current plugin configuration to frontend
type ConfigResponse struct {
	Success     bool         `json:"success"`
	TableConfig *TableConfig `json:"tableConfig,omitempty"`
	Error       string       `json:"error,omitempty"`
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
