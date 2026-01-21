package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Make sure Datasource implements required interfaces
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
)

// Datasource is the main plugin struct
type Datasource struct {
	backend.CallResourceHandler
	settings DatasourceSettings
	password string
}

// NewDatasource creates a new datasource instance
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	log.DefaultLogger.Info("Creating new SQL Server Datasource instance")

	var dsSettings DatasourceSettings
	if err := json.Unmarshal(settings.JSONData, &dsSettings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	// Set default port if not specified
	if dsSettings.Port == 0 {
		dsSettings.Port = 1433
	}

	// Set default schema if not specified
	if dsSettings.Schema == "" {
		dsSettings.Schema = "dbo"
	}

	password := settings.DecryptedSecureJSONData["password"]

	ds := &Datasource{
		settings: dsSettings,
		password: password,
	}

	// Setup resource handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/tables", ds.handleTables)
	mux.HandleFunc("/columns", ds.handleColumns)
	mux.HandleFunc("/values", ds.handleValues)
	ds.CallResourceHandler = httpadapter.New(mux)

	return ds, nil
}

// Dispose cleans up resources
func (d *Datasource) Dispose() {
	log.DefaultLogger.Info("Disposing SQL Server Datasource instance")
}

// CheckHealth validates the datasource configuration
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	log.DefaultLogger.Info("CheckHealth called")

	// Validate required settings
	if d.settings.Host == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Host is required",
		}, nil
	}

	if d.settings.Database == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Database is required",
		}, nil
	}

	client, err := NewSQLServerClient(d.settings, d.password)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Failed to connect: %s", err.Error()),
		}, nil
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Database ping failed: %s", err.Error()),
		}, nil
	}

	// Verify schema exists and count tables
	tables, err := client.GetTables(ctx, d.settings.Schema)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Failed to query schema '%s': %s", d.settings.Schema, err.Error()),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: fmt.Sprintf("Connected successfully. Found %d tables in '%s' schema.", len(tables), d.settings.Schema),
	}, nil
}

// QueryData handles incoming queries
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	log.DefaultLogger.Info("QueryData called", "queries", len(req.Queries))

	response := backend.NewQueryDataResponse()

	client, err := NewSQLServerClient(d.settings, d.password)
	if err != nil {
		for _, q := range req.Queries {
			response.Responses[q.RefID] = backend.DataResponse{
				Error: fmt.Errorf("failed to connect: %w", err),
			}
		}
		return response, nil
	}
	defer client.Close()

	for _, q := range req.Queries {
		response.Responses[q.RefID] = d.processQuery(ctx, client, q)
	}

	return response, nil
}

func (d *Datasource) processQuery(ctx context.Context, client *SQLServerClient, query backend.DataQuery) backend.DataResponse {
	var qm QueryModel
	if err := json.Unmarshal(query.JSON, &qm); err != nil {
		return backend.DataResponse{Error: fmt.Errorf("failed to parse query: %w", err)}
	}

	// Validate table name
	if qm.Table == "" {
		return backend.DataResponse{Error: fmt.Errorf("table is required")}
	}

	if !isValidIdentifier(qm.Table) {
		return backend.DataResponse{Error: fmt.Errorf("invalid table name: %s", qm.Table)}
	}

	// Build SQL query
	sqlQuery := d.buildQuery(qm, query.TimeRange)
	log.DefaultLogger.Debug("Executing query", "sql", sqlQuery)

	rows, err := client.ExecuteQuery(ctx, sqlQuery)
	if err != nil {
		return backend.DataResponse{Error: fmt.Errorf("query execution failed: %w", err)}
	}
	defer rows.Close()

	// Convert to data frame
	frame, err := d.rowsToFrame(rows, qm)
	if err != nil {
		return backend.DataResponse{Error: err}
	}

	return backend.DataResponse{Frames: []*data.Frame{frame}}
}

func (d *Datasource) buildQuery(qm QueryModel, timeRange backend.TimeRange) string {
	if qm.RawSQL != "" {
		return qm.RawSQL
	}

	// Build column list
	columns := "*"
	if len(qm.Columns) > 0 {
		var validColumns []string
		for _, col := range qm.Columns {
			if isValidIdentifier(col) {
				validColumns = append(validColumns, fmt.Sprintf("[%s]", col))
			}
		}
		if len(validColumns) > 0 {
			columns = strings.Join(validColumns, ", ")
		}
	}

	query := fmt.Sprintf("SELECT %s FROM [%s].[%s]", columns, d.settings.Schema, qm.Table)

	// Add filters
	var whereClauses []string
	for col, val := range qm.Filters {
		if isValidIdentifier(col) && val != "" {
			// Escape single quotes in value
			escapedVal := strings.ReplaceAll(val, "'", "''")
			whereClauses = append(whereClauses, fmt.Sprintf("[%s] = N'%s'", col, escapedVal))
		}
	}

	// Add time range filter if time series
	if qm.Format == "timeseries" && qm.TimeColumn != "" && isValidIdentifier(qm.TimeColumn) {
		whereClauses = append(whereClauses,
			fmt.Sprintf("[%s] >= '%s'", qm.TimeColumn, timeRange.From.Format(time.RFC3339)),
			fmt.Sprintf("[%s] <= '%s'", qm.TimeColumn, timeRange.To.Format(time.RFC3339)),
		)
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Add ORDER BY for time series
	if qm.Format == "timeseries" && qm.TimeColumn != "" && isValidIdentifier(qm.TimeColumn) {
		query += fmt.Sprintf(" ORDER BY [%s]", qm.TimeColumn)
	}

	return query
}

func (d *Datasource) rowsToFrame(rows *sql.Rows, qm QueryModel) (*data.Frame, error) {
	columns, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	frame := data.NewFrame(qm.Table)

	// Create fields based on column types
	fields := make([]*data.Field, len(columns))
	timeFieldIndex := -1
	for i, col := range columns {
		fields[i] = d.createField(col)
		// Mark time column for time series
		if qm.Format == "timeseries" && qm.TimeColumn != "" && col.Name() == qm.TimeColumn {
			timeFieldIndex = i
		}
		frame.Fields = append(frame.Fields, fields[i])
	}

	// Scan rows
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		for i, val := range values {
			d.appendValue(frame.Fields[i], val, columns[i].DatabaseTypeName())
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Set frame meta for time series
	if qm.Format == "timeseries" && timeFieldIndex >= 0 {
		// Reorder fields so time column is first (required by Grafana)
		if timeFieldIndex > 0 {
			timeField := frame.Fields[timeFieldIndex]
			newFields := make([]*data.Field, 0, len(frame.Fields))
			newFields = append(newFields, timeField)
			for i, f := range frame.Fields {
				if i != timeFieldIndex {
					newFields = append(newFields, f)
				}
			}
			frame.Fields = newFields
		}

		frame.Meta = &data.FrameMeta{
			Type:                   data.FrameTypeTimeSeriesLong,
			PreferredVisualization: data.VisTypeGraph,
		}
	}

	return frame, nil
}

func (d *Datasource) createField(col *sql.ColumnType) *data.Field {
	dbType := col.DatabaseTypeName()

	switch dbType {
	case "INT", "BIGINT", "SMALLINT", "TINYINT":
		return data.NewField(col.Name(), nil, []*int64{})
	case "FLOAT", "REAL", "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
		return data.NewField(col.Name(), nil, []*float64{})
	case "DATETIME", "DATETIME2", "DATE", "TIME", "SMALLDATETIME", "DATETIMEOFFSET":
		return data.NewField(col.Name(), nil, []*time.Time{})
	case "BIT":
		return data.NewField(col.Name(), nil, []*bool{})
	default:
		return data.NewField(col.Name(), nil, []*string{})
	}
}

func (d *Datasource) appendValue(field *data.Field, val interface{}, dbType string) {
	if val == nil {
		switch dbType {
		case "INT", "BIGINT", "SMALLINT", "TINYINT":
			field.Append((*int64)(nil))
		case "FLOAT", "REAL", "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
			field.Append((*float64)(nil))
		case "DATETIME", "DATETIME2", "DATE", "TIME", "SMALLDATETIME", "DATETIMEOFFSET":
			field.Append((*time.Time)(nil))
		case "BIT":
			field.Append((*bool)(nil))
		default:
			field.Append((*string)(nil))
		}
		return
	}

	switch dbType {
	case "INT", "BIGINT", "SMALLINT", "TINYINT":
		if v, ok := val.(int64); ok {
			field.Append(&v)
		} else {
			field.Append((*int64)(nil))
		}
	case "FLOAT", "REAL", "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
		if v, ok := val.(float64); ok {
			field.Append(&v)
		} else {
			field.Append((*float64)(nil))
		}
	case "DATETIME", "DATETIME2", "DATE", "TIME", "SMALLDATETIME", "DATETIMEOFFSET":
		if v, ok := val.(time.Time); ok {
			field.Append(&v)
		} else {
			field.Append((*time.Time)(nil))
		}
	case "BIT":
		if v, ok := val.(bool); ok {
			field.Append(&v)
		} else {
			field.Append((*bool)(nil))
		}
	default:
		s := fmt.Sprintf("%v", val)
		field.Append(&s)
	}
}
