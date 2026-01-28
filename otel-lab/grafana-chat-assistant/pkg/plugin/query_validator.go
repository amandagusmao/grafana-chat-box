package plugin

import (
	"regexp"
	"strings"
)

// QueryValidationResult represents the result of query validation
type QueryValidationResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// ValidatePromQLQuery validates a PromQL query for potential issues
func ValidatePromQLQuery(query string) QueryValidationResult {
	result := QueryValidationResult{Valid: true}

	query = strings.TrimSpace(query)
	if query == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Query não pode ser vazia")
		return result
	}

	// Check query length (very long queries are suspicious)
	if len(query) > 2000 {
		result.Valid = false
		result.Errors = append(result.Errors, "Query muito longa (máximo 2000 caracteres)")
		return result
	}

	// Check for dangerous patterns that could overload Prometheus

	// 1. High cardinality - selecting all values without aggregation
	if strings.Contains(query, "{__name__=~\".+\"}") || strings.Contains(query, "{__name__=~\".*\"}") {
		result.Valid = false
		result.Errors = append(result.Errors, "Query seleciona todas as métricas - isso pode sobrecarregar o Prometheus")
		return result
	}

	// 2. Very broad regex that matches too much
	broadRegexPatterns := []string{
		`=~".*"`,
		`=~".+"`,
		`=~"[^"]*\.\*[^"]*"`,
	}
	for _, pattern := range broadRegexPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			// Allow if it's combined with other filters
			if !strings.Contains(query, ",") && !strings.Contains(query, "by") {
				result.Warnings = append(result.Warnings, "Query usa regex muito amplo - considere filtros mais específicos")
			}
		}
	}

	// 3. Long time ranges without rate/increase/delta
	longRanges := []string{"[30d]", "[60d]", "[90d]", "[7d]", "[14d]", "[1w]", "[2w]", "[1M]", "[1y]"}
	hasLongRange := false
	for _, lr := range longRanges {
		if strings.Contains(query, lr) {
			hasLongRange = true
			break
		}
	}

	if hasLongRange {
		// Check if it has rate, increase, delta, or other range functions
		rangeFunctions := []string{"rate(", "irate(", "increase(", "delta(", "idelta(", "deriv(", "avg_over_time(", "sum_over_time(", "max_over_time(", "min_over_time("}
		hasRangeFunc := false
		for _, fn := range rangeFunctions {
			if strings.Contains(query, fn) {
				hasRangeFunc = true
				break
			}
		}

		if !hasRangeFunc {
			result.Warnings = append(result.Warnings, "Query com range longo sem função de agregação temporal pode ser pesada")
		}
	}

	// 4. Check for potentially heavy operations without proper aggregation
	heavyOps := []string{"histogram_quantile(", "label_replace(", "label_join("}
	for _, op := range heavyOps {
		if strings.Contains(query, op) {
			// These are fine but should have proper aggregation
			if !strings.Contains(query, "by (") && !strings.Contains(query, "by(") &&
				!strings.Contains(query, "without (") && !strings.Contains(query, "without(") {
				result.Warnings = append(result.Warnings, "Query usa operação pesada sem agregação 'by' ou 'without'")
			}
		}
	}

	// 5. Subqueries with short step and long range
	subqueryPattern := regexp.MustCompile(`\[[0-9]+[smhdwy]\s*:\s*[0-9]+[smhdwy]\]`)
	if subqueryPattern.MatchString(query) {
		result.Warnings = append(result.Warnings, "Query contém subquery - verifique se o step é adequado para o range")
	}

	// 6. Check for missing rate() on counters (heuristic based on common naming)
	counterPatterns := []string{"_total", "_count", "_bucket", "_sum"}
	hasCounter := false
	for _, cp := range counterPatterns {
		if strings.Contains(query, cp) {
			hasCounter = true
			break
		}
	}

	if hasCounter {
		if !strings.Contains(query, "rate(") && !strings.Contains(query, "increase(") &&
			!strings.Contains(query, "irate(") && !strings.Contains(query, "delta(") {
			// Allow histogram_quantile which handles counters internally
			if !strings.Contains(query, "histogram_quantile(") {
				result.Warnings = append(result.Warnings, "Query usa contador sem rate()/increase() - counters devem usar funções de rate")
			}
		}
	}

	return result
}

// ValidateLogQLQuery validates a LogQL query for potential issues
func ValidateLogQLQuery(query string) QueryValidationResult {
	result := QueryValidationResult{Valid: true}

	query = strings.TrimSpace(query)
	if query == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Query não pode ser vazia")
		return result
	}

	// Check query length
	if len(query) > 2000 {
		result.Valid = false
		result.Errors = append(result.Errors, "Query muito longa (máximo 2000 caracteres)")
		return result
	}

	// 1. Must have stream selector (starts with {)
	if !strings.HasPrefix(query, "{") {
		result.Warnings = append(result.Warnings, "Query LogQL deve começar com um stream selector {label=\"value\"}")
	}

	// 2. Check for empty stream selector which selects ALL logs
	if strings.HasPrefix(query, "{}") {
		result.Valid = false
		result.Errors = append(result.Errors, "Query seleciona TODOS os logs - adicione filtros de label")
		return result
	}

	// 3. Check for very broad regex in stream selector
	if strings.Contains(query, `=~".*"`) || strings.Contains(query, `=~".+"`) {
		result.Warnings = append(result.Warnings, "Query usa regex muito amplo no stream selector")
	}

	// 4. Check for missing label filters
	// Extract stream selector
	if idx := strings.Index(query, "}"); idx > 0 {
		selector := query[1:idx]
		if !strings.Contains(selector, "=") {
			result.Warnings = append(result.Warnings, "Stream selector sem filtros de label pode retornar muitos logs")
		}
	}

	return result
}

// ValidatePanelQueries validates all queries in a dashboard panel configuration
func ValidatePanelQueries(panels []PanelConfig) QueryValidationResult {
	aggregateResult := QueryValidationResult{Valid: true}

	for i, panel := range panels {
		var result QueryValidationResult

		// Determine query type based on panel type or explicit query_type
		switch panel.QueryType {
		case "logql":
			result = ValidateLogQLQuery(panel.Query)
		case "traceql":
			// TraceQL validation is simpler for now
			if panel.Query == "" {
				result.Valid = false
				result.Errors = append(result.Errors, "Query não pode ser vazia")
			}
		default:
			// Default to PromQL
			if panel.Type == "logs" {
				result = ValidateLogQLQuery(panel.Query)
			} else {
				result = ValidatePromQLQuery(panel.Query)
			}
		}

		// Aggregate results
		if !result.Valid {
			aggregateResult.Valid = false
		}

		for _, err := range result.Errors {
			aggregateResult.Errors = append(aggregateResult.Errors,
				strings.ReplaceAll(err, "Query", "Painel "+panel.Title+" ("+string(rune('1'+i))+"): Query"))
		}

		for _, warn := range result.Warnings {
			aggregateResult.Warnings = append(aggregateResult.Warnings,
				strings.ReplaceAll(warn, "Query", "Painel "+panel.Title+": Query"))
		}
	}

	return aggregateResult
}
