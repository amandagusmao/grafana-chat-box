package plugin

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"

	_ "github.com/microsoft/go-mssqldb"
)

// SQLServerClient handles database connections
type SQLServerClient struct {
	db *sql.DB
}

// NewSQLServerClient creates a new client with connection
func NewSQLServerClient(settings DatasourceSettings, password string) (*SQLServerClient, error) {
	query := url.Values{}
	query.Add("database", settings.Database)
	if settings.Encrypt != "" {
		query.Add("encrypt", settings.Encrypt)
	} else {
		query.Add("encrypt", "disable")
	}
	query.Add("TrustServerCertificate", "true")

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(settings.User, password),
		Host:     fmt.Sprintf("%s:%d", settings.Host, settings.Port),
		RawQuery: query.Encode(),
	}

	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	return &SQLServerClient{db: db}, nil
}

// GetTables returns all tables in the specified schema
func (c *SQLServerClient) GetTables(ctx context.Context, schema string) ([]TableInfo, error) {
	if !isValidIdentifier(schema) {
		return nil, fmt.Errorf("invalid schema name: %s", schema)
	}

	query := `
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = @schema AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`
	rows, err := c.db.QueryContext(ctx, query, sql.Named("schema", schema))
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, TableInfo{Name: name, Schema: schema})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	return tables, nil
}

// GetTableColumns returns columns for a specific table in the specified schema
func (c *SQLServerClient) GetTableColumns(ctx context.Context, schema, table string) ([]ColumnInfo, error) {
	// Validate identifiers to prevent SQL injection
	if !isValidIdentifier(schema) {
		return nil, fmt.Errorf("invalid schema name: %s", schema)
	}
	if !isValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	query := `
		SELECT COLUMN_NAME, DATA_TYPE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = @schema AND TABLE_NAME = @table
		ORDER BY ORDINAL_POSITION
	`
	rows, err := c.db.QueryContext(ctx, query, sql.Named("schema", schema), sql.Named("table", table))
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		if err := rows.Scan(&col.Name, &col.DataType); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	return columns, nil
}

// GetDistinctValues returns distinct values for a column (limited to top 1000)
func (c *SQLServerClient) GetDistinctValues(ctx context.Context, schema, table, column string) ([]DistinctValue, error) {
	// Validate identifiers to prevent SQL injection
	if !isValidIdentifier(schema) {
		return nil, fmt.Errorf("invalid schema name: %s", schema)
	}
	if !isValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}
	if !isValidIdentifier(column) {
		return nil, fmt.Errorf("invalid column name: %s", column)
	}

	// Using dynamic SQL with validated identifiers
	query := fmt.Sprintf(`
		SELECT TOP 1000 CAST([%s] AS NVARCHAR(MAX)) as val, COUNT(*) as cnt
		FROM [%s].[%s]
		WHERE [%s] IS NOT NULL
		GROUP BY [%s]
		ORDER BY COUNT(*) DESC
	`, column, schema, table, column, column)

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct values: %w", err)
	}
	defer rows.Close()

	var values []DistinctValue
	for rows.Next() {
		var v DistinctValue
		if err := rows.Scan(&v.Value, &v.Count); err != nil {
			return nil, fmt.Errorf("failed to scan value: %w", err)
		}
		values = append(values, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating values: %w", err)
	}

	return values, nil
}

// ExecuteQuery runs a data query and returns rows
func (c *SQLServerClient) ExecuteQuery(ctx context.Context, query string) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query)
}

// Close closes the database connection
func (c *SQLServerClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Ping tests the connection
func (c *SQLServerClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// isValidIdentifier checks if a string is a valid SQL identifier
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	// Allow alphanumeric, underscore, and common characters
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, s)
	return matched
}
