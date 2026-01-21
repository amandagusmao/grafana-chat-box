package plugin

// DatasourceSettings holds the connection configuration
type DatasourceSettings struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Encrypt  string `json:"encrypt"` // disable, false, true
	Schema   string `json:"schema"`  // schema to query tables from (default: dbo)
}

// QueryModel represents the query from the frontend
type QueryModel struct {
	Table       string            `json:"table"`
	Columns     []string          `json:"columns"`     // Selected columns
	Filters     map[string]string `json:"filters"`     // Column -> value filters
	Format      string            `json:"format"`      // "table" or "timeseries"
	TimeColumn  string            `json:"timeColumn"`  // For time series
	ValueColumn string            `json:"valueColumn"` // For time series
	RawSQL      string            `json:"rawSQL"`      // Optional raw SQL mode
}

// TableInfo represents a table in the kafka schema
type TableInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

// ColumnInfo represents a column in a table
type ColumnInfo struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

// DistinctValue represents a distinct value for a column
type DistinctValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}
