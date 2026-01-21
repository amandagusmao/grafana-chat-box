package plugin

import (
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// handleTables returns all tables in configured schema
func (d *Datasource) handleTables(w http.ResponseWriter, r *http.Request) {
	log.DefaultLogger.Info("handleTables called", "schema", d.settings.Schema)
	w.Header().Set("Content-Type", "application/json")

	client, err := NewSQLServerClient(d.settings, d.password)
	if err != nil {
		log.DefaultLogger.Error("Failed to connect", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	tables, err := client.GetTables(r.Context(), d.settings.Schema)
	if err != nil {
		log.DefaultLogger.Error("Failed to get tables", "error", err, "schema", d.settings.Schema)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if tables == nil {
		tables = []TableInfo{}
	}

	if err := json.NewEncoder(w).Encode(tables); err != nil {
		log.DefaultLogger.Error("Failed to encode response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleColumns returns columns for a table
func (d *Datasource) handleColumns(w http.ResponseWriter, r *http.Request) {
	log.DefaultLogger.Info("handleColumns called", "schema", d.settings.Schema)
	w.Header().Set("Content-Type", "application/json")

	table := r.URL.Query().Get("table")
	if table == "" {
		http.Error(w, "table parameter required", http.StatusBadRequest)
		return
	}

	if !isValidIdentifier(table) {
		http.Error(w, "invalid table name", http.StatusBadRequest)
		return
	}

	client, err := NewSQLServerClient(d.settings, d.password)
	if err != nil {
		log.DefaultLogger.Error("Failed to connect", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	columns, err := client.GetTableColumns(r.Context(), d.settings.Schema, table)
	if err != nil {
		log.DefaultLogger.Error("Failed to get columns", "error", err, "schema", d.settings.Schema, "table", table)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if columns == nil {
		columns = []ColumnInfo{}
	}

	if err := json.NewEncoder(w).Encode(columns); err != nil {
		log.DefaultLogger.Error("Failed to encode response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleValues returns distinct values for a column
func (d *Datasource) handleValues(w http.ResponseWriter, r *http.Request) {
	log.DefaultLogger.Info("handleValues called", "schema", d.settings.Schema)
	w.Header().Set("Content-Type", "application/json")

	table := r.URL.Query().Get("table")
	column := r.URL.Query().Get("column")

	if table == "" || column == "" {
		http.Error(w, "table and column parameters required", http.StatusBadRequest)
		return
	}

	if !isValidIdentifier(table) {
		http.Error(w, "invalid table name", http.StatusBadRequest)
		return
	}

	if !isValidIdentifier(column) {
		http.Error(w, "invalid column name", http.StatusBadRequest)
		return
	}

	client, err := NewSQLServerClient(d.settings, d.password)
	if err != nil {
		log.DefaultLogger.Error("Failed to connect", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	values, err := client.GetDistinctValues(r.Context(), d.settings.Schema, table, column)
	if err != nil {
		log.DefaultLogger.Error("Failed to get distinct values", "error", err, "schema", d.settings.Schema, "table", table, "column", column)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if values == nil {
		values = []DistinctValue{}
	}

	if err := json.NewEncoder(w).Encode(values); err != nil {
		log.DefaultLogger.Error("Failed to encode response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
