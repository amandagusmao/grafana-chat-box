package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// GrafanaService handles Grafana API interactions
type GrafanaService struct {
	url            string
	token          string
	impersonateUser string // User to impersonate for audit purposes (login or email)
}

// DashboardResult represents the result of dashboard creation
type DashboardResult struct {
	ID      int64  `json:"id"`
	UID     string `json:"uid"`
	URL     string `json:"url"`
	Status  string `json:"status"`
	Version int    `json:"version"`
}

// Folder represents a Grafana folder
type Folder struct {
	ID    int64  `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
}

// NewGrafanaService creates a new Grafana service instance
func NewGrafanaService(url, token string) *GrafanaService {
	return &GrafanaService{
		url:   strings.TrimRight(url, "/"),
		token: token,
	}
}

// NewGrafanaServiceWithUser creates a new Grafana service with user impersonation for audit
func NewGrafanaServiceWithUser(url, token, impersonateUser string) *GrafanaService {
	return &GrafanaService{
		url:             strings.TrimRight(url, "/"),
		token:           token,
		impersonateUser: impersonateUser,
	}
}

// GetOrCreateFolder gets an existing folder or creates a new one
func (s *GrafanaService) GetOrCreateFolder(folderName string) (int64, error) {
	// Get existing folders
	req, err := http.NewRequest("GET", s.url+"/api/folders", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to fetch folders: %s - %s", resp.Status, string(body))
	}

	var folders []Folder
	if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
		return 0, err
	}

	// Check if folder already exists
	for _, f := range folders {
		if f.Title == folderName {
			return f.ID, nil
		}
	}

	// Create new folder
	folderUID := strings.ToLower(folderName)
	folderUID = strings.ReplaceAll(folderUID, " ", "-")

	folderData := map[string]string{
		"title": folderName,
		"uid":   folderUID,
	}
	folderJSON, _ := json.Marshal(folderData)

	req, err = http.NewRequest("POST", s.url+"/api/folders", bytes.NewBuffer(folderJSON))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to create folder: %s - %s", resp.Status, string(body))
	}

	var newFolder Folder
	if err := json.NewDecoder(resp.Body).Decode(&newFolder); err != nil {
		return 0, err
	}

	return newFolder.ID, nil
}

// CreateDashboard creates a new dashboard in Grafana (legacy method)
func (s *GrafanaService) CreateDashboard(data DashboardData) (*DashboardResult, error) {
	log.DefaultLogger.Info("Creating dashboard (legacy)", "title", data.Title, "datasource", data.Datasource)

	var folderID int64 = 0 // Default: General (root level)

	if data.Folder != "" && strings.ToLower(data.Folder) != "general" {
		var err error
		folderID, err = s.GetOrCreateFolder(data.Folder)
		if err != nil {
			log.DefaultLogger.Warn("Failed to get/create folder, using General", "error", err)
		}
	}

	// Create panels for each metric
	panels := make([]map[string]interface{}, len(data.Metrics))
	for i, metric := range data.Metrics {
		panels[i] = map[string]interface{}{
			"id":    i + 1,
			"type":  "timeseries",
			"title": metric,
			"datasource": map[string]interface{}{
				"type": "prometheus",
				"uid":  data.Datasource,
			},
			"targets": []map[string]interface{}{
				{
					"expr":  metric,
					"refId": string(rune('A' + i)),
					"datasource": map[string]interface{}{
						"type": "prometheus",
						"uid":  data.Datasource,
					},
				},
			},
			"gridPos": map[string]interface{}{
				"x": (i % 2) * 12,
				"y": (i / 2) * 8,
				"w": 12,
				"h": 8,
			},
			"fieldConfig": map[string]interface{}{
				"defaults": map[string]interface{}{
					"unit": "short",
					"custom": map[string]interface{}{
						"drawStyle":         "line",
						"lineInterpolation": "linear",
						"barAlignment":      0,
						"lineWidth":         1,
						"fillOpacity":       0.1,
						"gradientMode":      "none",
					},
				},
			},
		}
	}

	dashboard := map[string]interface{}{
		"dashboard": map[string]interface{}{
			"id":       nil,
			"title":    data.Title,
			"tags":     []string{"ai-generated"},
			"timezone": "browser",
			"panels":   panels,
			"time": map[string]string{
				"from": "now-1h",
				"to":   "now",
			},
			"timepicker":    map[string]interface{}{},
			"templating":    map[string]interface{}{"list": []interface{}{}},
			"annotations":   map[string]interface{}{"list": []interface{}{}},
			"refresh":       "5s",
			"schemaVersion": 27,
			"version":       0,
			"links":         []interface{}{},
		},
		"folderId":  folderID,
		"overwrite": false,
	}

	dashboardJSON, err := json.Marshal(dashboard)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", s.url+"/api/dashboards/db", bytes.NewBuffer(dashboardJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create dashboard: %s - %s", resp.Status, string(body))
	}

	var result DashboardResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	log.DefaultLogger.Info("Dashboard created successfully", "uid", result.UID)
	return &result, nil
}

// CreateAdvancedDashboard creates a dashboard with advanced configuration
func (s *GrafanaService) CreateAdvancedDashboard(data *AdvancedDashData) (*DashboardResult, error) {
	log.DefaultLogger.Info("Creating advanced dashboard", "title", data.Title, "panels", len(data.Panels))

	var folderID int64 = 0

	if data.Folder != "" && strings.ToLower(data.Folder) != "general" {
		var err error
		folderID, err = s.GetOrCreateFolder(data.Folder)
		if err != nil {
			log.DefaultLogger.Warn("Failed to get/create folder, using General", "error", err)
		}
	}

	// Build panels
	panels := make([]map[string]interface{}, len(data.Panels))
	currentY := 0

	for i, panelCfg := range data.Panels {
		width := panelCfg.Width
		if width == 0 {
			width = 12 // Default width
		}
		height := panelCfg.Height
		if height == 0 {
			height = 8 // Default height
		}

		// Calculate grid position
		x := (i % 2) * 12
		if i%2 == 0 && i > 0 {
			currentY += height
		}

		panel := s.buildPanel(i+1, panelCfg, data.DatasourceUID, x, currentY, width, height)
		panels[i] = panel
	}

	// Build variables/templating
	templating := s.buildTemplating(data.Variables, data.DatasourceUID)

	// Build tags
	tags := data.Tags
	if tags == nil {
		tags = []string{}
	}

	// Add ai-generated tag if not present
	hasAITag := false
	for _, t := range tags {
		if t == "ai-generated" {
			hasAITag = true
			break
		}
	}
	if !hasAITag {
		tags = append(tags, "ai-generated")
	}

	// Add requester tag for audit purposes (visible in dashboard list)
	if data.RequestedBy != nil && data.RequestedBy.Email != "" {
		// Sanitize email for use as tag (keep alphanumeric, @, ., -, _)
		safeEmail := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '@' {
				return r
			}
			return '-'
		}, data.RequestedBy.Email)
		requesterTag := fmt.Sprintf("solicitado-por:%s", safeEmail)
		tags = append(tags, requesterTag)
		log.DefaultLogger.Info("Added requester tag", "tag", requesterTag)
	}

	// Build time range
	timeFrom := "now-1h"
	timeTo := "now"
	if data.TimeRange != nil {
		if data.TimeRange.From != "" {
			timeFrom = data.TimeRange.From
		}
		if data.TimeRange.To != "" {
			timeTo = data.TimeRange.To
		}
	}

	// Build refresh
	refresh := "10s"
	if data.Refresh != "" {
		refresh = data.Refresh
	}

	// Build description with requester information for audit purposes
	description := data.Description
	if data.RequestedBy != nil {
		requesterInfo := ""
		if data.RequestedBy.Login != "" {
			requesterInfo = data.RequestedBy.Login
		}
		if data.RequestedBy.Email != "" {
			if requesterInfo != "" {
				requesterInfo += " (" + data.RequestedBy.Email + ")"
			} else {
				requesterInfo = data.RequestedBy.Email
			}
		}
		if requesterInfo != "" {
			if description != "" {
				description += "\n\n---\n"
			}
			description += fmt.Sprintf("Dashboard criado via AI Assistant por: %s", requesterInfo)
		}
		log.DefaultLogger.Info("Dashboard requested by user", "login", data.RequestedBy.Login, "email", data.RequestedBy.Email)
	}

	// Build annotations with requester information
	annotations := map[string]interface{}{"list": []interface{}{}}
	if data.RequestedBy != nil && (data.RequestedBy.Login != "" || data.RequestedBy.Email != "") {
		// Add a custom annotation with creation metadata
		annotations = map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{
					"builtIn":    0,
					"enable":     false, // Hidden annotation, just for metadata
					"hide":       true,
					"iconColor":  "rgba(0, 211, 255, 1)",
					"name":       "AI Creation Metadata",
					"type":       "dashboard",
					"datasource": map[string]interface{}{"type": "grafana", "uid": "-- Grafana --"},
					"target": map[string]interface{}{
						"limit":    100,
						"matchAny": false,
						"tags":     []string{"ai-metadata"},
						"type":     "dashboard",
					},
					// Custom fields for audit
					"_ai_created":      true,
					"_requested_by":    data.RequestedBy.Login,
					"_requested_email": data.RequestedBy.Email,
				},
			},
		}
	}

	dashboard := map[string]interface{}{
		"dashboard": map[string]interface{}{
			"id":          nil,
			"title":       data.Title,
			"description": description,
			"tags":        tags,
			"timezone":    "browser",
			"panels":      panels,
			"time": map[string]string{
				"from": timeFrom,
				"to":   timeTo,
			},
			"timepicker":    map[string]interface{}{},
			"templating":    templating,
			"annotations":   annotations,
			"refresh":       refresh,
			"schemaVersion": 39,
			"version":       0,
			"links":         []interface{}{},
		},
		"folderId":  folderID,
		"overwrite": false,
	}

	dashboardJSON, err := json.Marshal(dashboard)
	if err != nil {
		return nil, err
	}

	log.DefaultLogger.Debug("Dashboard JSON", "json", string(dashboardJSON))

	req, err := http.NewRequest("POST", s.url+"/api/dashboards/db", bytes.NewBuffer(dashboardJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	// Add impersonation header for audit purposes
	// This makes Grafana record the dashboard as created by the specified user
	// Note: Requires Service Account with admin permissions
	if s.impersonateUser != "" {
		req.Header.Set("X-Grafana-User", s.impersonateUser)
		log.DefaultLogger.Info("Using user impersonation for dashboard creation", "user", s.impersonateUser)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create dashboard: %s - %s", resp.Status, string(body))
	}

	var result DashboardResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	log.DefaultLogger.Info("Advanced dashboard created successfully", "uid", result.UID, "title", data.Title, "requestedBy", s.impersonateUser)
	return &result, nil
}

// buildPanel builds a single panel configuration
func (s *GrafanaService) buildPanel(id int, cfg PanelConfig, datasourceUID string, x, y, w, h int) map[string]interface{} {
	panel := map[string]interface{}{
		"id":    id,
		"type":  cfg.Type,
		"title": cfg.Title,
		"datasource": map[string]interface{}{
			"type": "prometheus",
			"uid":  datasourceUID,
		},
		"gridPos": map[string]interface{}{
			"x": x,
			"y": y,
			"w": w,
			"h": h,
		},
	}

	if cfg.Description != "" {
		panel["description"] = cfg.Description
	}

	// Build targets based on panel type
	targets := s.buildTargets(cfg, datasourceUID)
	panel["targets"] = targets

	// Build field config based on panel type
	fieldConfig := s.buildFieldConfig(cfg)
	panel["fieldConfig"] = fieldConfig

	// Build options based on panel type
	options := s.buildPanelOptions(cfg)
	panel["options"] = options

	return panel
}

// buildTargets builds query targets for a panel
func (s *GrafanaService) buildTargets(cfg PanelConfig, datasourceUID string) []map[string]interface{} {
	dsType := "prometheus"
	if cfg.QueryType == "logql" {
		dsType = "loki"
	} else if cfg.QueryType == "traceql" {
		dsType = "tempo"
	}

	target := map[string]interface{}{
		"refId": "A",
		"datasource": map[string]interface{}{
			"type": dsType,
			"uid":  datasourceUID,
		},
	}

	switch dsType {
	case "prometheus":
		target["expr"] = cfg.Query
		target["legendFormat"] = "{{instance}}"
		target["instant"] = cfg.Type == "stat" || cfg.Type == "gauge"
		target["range"] = cfg.Type != "stat" && cfg.Type != "gauge"
	case "loki":
		target["expr"] = cfg.Query
	case "tempo":
		target["query"] = cfg.Query
		target["queryType"] = "traceql"
	}

	return []map[string]interface{}{target}
}

// buildFieldConfig builds field configuration for a panel
func (s *GrafanaService) buildFieldConfig(cfg PanelConfig) map[string]interface{} {
	defaults := map[string]interface{}{}

	// Set unit
	if cfg.Unit != "" {
		defaults["unit"] = cfg.Unit
	} else {
		defaults["unit"] = "short"
	}

	// Set thresholds
	if len(cfg.Thresholds) > 0 {
		steps := make([]map[string]interface{}, 0, len(cfg.Thresholds)+1)

		// Always start with a base step
		steps = append(steps, map[string]interface{}{
			"color": "green",
			"value": nil,
		})

		for _, t := range cfg.Thresholds {
			steps = append(steps, map[string]interface{}{
				"color": s.normalizeColor(t.Color),
				"value": t.Value,
			})
		}

		defaults["thresholds"] = map[string]interface{}{
			"mode":  "absolute",
			"steps": steps,
		}
	} else {
		// Default thresholds
		defaults["thresholds"] = map[string]interface{}{
			"mode": "absolute",
			"steps": []map[string]interface{}{
				{"color": "green", "value": nil},
				{"color": "yellow", "value": 70},
				{"color": "red", "value": 90},
			},
		}
	}

	// Panel-type specific custom settings
	switch cfg.Type {
	case "timeseries":
		defaults["custom"] = map[string]interface{}{
			"drawStyle":         "line",
			"lineInterpolation": "smooth",
			"barAlignment":      0,
			"lineWidth":         2,
			"fillOpacity":       10,
			"gradientMode":      "opacity",
			"showPoints":        "never",
			"spanNulls":         true,
			"axisCenteredZero":  false,
			"axisColorMode":     "text",
			"scaleDistribution": map[string]interface{}{"type": "linear"},
		}
	case "bargauge", "gauge":
		defaults["color"] = map[string]interface{}{
			"mode": "thresholds",
		}
	case "stat":
		defaults["color"] = map[string]interface{}{
			"mode": "thresholds",
		}
	}

	return map[string]interface{}{
		"defaults":  defaults,
		"overrides": []interface{}{},
	}
}

// buildPanelOptions builds panel-specific options
func (s *GrafanaService) buildPanelOptions(cfg PanelConfig) map[string]interface{} {
	switch cfg.Type {
	case "timeseries":
		return map[string]interface{}{
			"tooltip": map[string]interface{}{
				"mode": "multi",
				"sort": "desc",
			},
			"legend": map[string]interface{}{
				"showLegend":  true,
				"displayMode": "table",
				"placement":   "bottom",
				"calcs":       []string{"mean", "max", "last"},
			},
		}
	case "stat":
		return map[string]interface{}{
			"reduceOptions": map[string]interface{}{
				"values": false,
				"calcs":  []string{"lastNotNull"},
				"fields": "",
			},
			"orientation":   "auto",
			"textMode":      "auto",
			"colorMode":     "value",
			"graphMode":     "area",
			"justifyMode":   "auto",
			"showPercentChange": false,
			"wideLayout":    true,
		}
	case "gauge":
		return map[string]interface{}{
			"reduceOptions": map[string]interface{}{
				"values": false,
				"calcs":  []string{"lastNotNull"},
				"fields": "",
			},
			"showThresholdLabels":  false,
			"showThresholdMarkers": true,
			"orientation":          "auto",
		}
	case "bargauge":
		return map[string]interface{}{
			"reduceOptions": map[string]interface{}{
				"values": false,
				"calcs":  []string{"lastNotNull"},
				"fields": "",
			},
			"orientation":   "horizontal",
			"displayMode":   "gradient",
			"showUnfilled":  true,
			"minVizWidth":   0,
			"minVizHeight":  10,
		}
	case "table":
		return map[string]interface{}{
			"showHeader": true,
			"footer": map[string]interface{}{
				"show":         false,
				"reducer":      []string{"sum"},
				"countRows":    false,
				"enablePagination": false,
			},
		}
	case "heatmap":
		return map[string]interface{}{
			"calculate": false,
			"cellGap":   1,
			"color": map[string]interface{}{
				"mode":   "scheme",
				"scheme": "Oranges",
				"steps":  64,
			},
			"yAxis": map[string]interface{}{
				"axisPlacement": "left",
			},
		}
	case "logs":
		return map[string]interface{}{
			"showTime":           true,
			"showLabels":         true,
			"showCommonLabels":   false,
			"wrapLogMessage":     true,
			"prettifyLogMessage": false,
			"enableLogDetails":   true,
			"dedupStrategy":      "none",
			"sortOrder":          "Descending",
		}
	case "piechart":
		return map[string]interface{}{
			"reduceOptions": map[string]interface{}{
				"values": false,
				"calcs":  []string{"lastNotNull"},
				"fields": "",
			},
			"pieType":     "pie",
			"displayLabels": []string{"name", "value"},
			"legend": map[string]interface{}{
				"showLegend":  true,
				"displayMode": "table",
				"placement":   "right",
				"values":      []string{"value", "percent"},
			},
		}
	default:
		return map[string]interface{}{}
	}
}

// buildTemplating builds the templating/variables section
func (s *GrafanaService) buildTemplating(variables []VariableConfig, defaultDatasourceUID string) map[string]interface{} {
	if len(variables) == 0 {
		return map[string]interface{}{"list": []interface{}{}}
	}

	list := make([]map[string]interface{}, len(variables))

	for i, v := range variables {
		variable := map[string]interface{}{
			"name":       v.Name,
			"label":      v.Label,
			"type":       v.Type,
			"hide":       0,
			"includeAll": v.IncludeAll,
			"multi":      v.Multi,
		}

		if v.Label == "" {
			variable["label"] = v.Name
		}

		switch v.Type {
		case "query":
			dsUID := v.Datasource
			if dsUID == "" {
				dsUID = defaultDatasourceUID
			}
			variable["datasource"] = map[string]interface{}{
				"type": "prometheus",
				"uid":  dsUID,
			}
			variable["query"] = map[string]interface{}{
				"query": v.Query,
				"refId": "StandardVariableQuery",
			}
			variable["refresh"] = v.Refresh
			if variable["refresh"] == 0 {
				variable["refresh"] = 1 // On dashboard load
			}
			variable["sort"] = 1 // Alphabetical (asc)
		case "custom":
			if v.Options != "" {
				options := strings.Split(v.Options, ",")
				customOptions := make([]map[string]interface{}, len(options))
				for j, opt := range options {
					opt = strings.TrimSpace(opt)
					customOptions[j] = map[string]interface{}{
						"selected": j == 0,
						"text":     opt,
						"value":    opt,
					}
				}
				variable["options"] = customOptions
				variable["query"] = v.Options
			}
		case "interval":
			variable["query"] = "1m,5m,10m,30m,1h,6h,12h,1d,7d"
			variable["options"] = []map[string]interface{}{
				{"selected": false, "text": "1m", "value": "1m"},
				{"selected": true, "text": "5m", "value": "5m"},
				{"selected": false, "text": "10m", "value": "10m"},
				{"selected": false, "text": "30m", "value": "30m"},
				{"selected": false, "text": "1h", "value": "1h"},
				{"selected": false, "text": "6h", "value": "6h"},
				{"selected": false, "text": "12h", "value": "12h"},
				{"selected": false, "text": "1d", "value": "1d"},
				{"selected": false, "text": "7d", "value": "7d"},
			}
			variable["auto"] = false
			variable["auto_count"] = 30
			variable["auto_min"] = "10s"
		case "datasource":
			variable["query"] = "prometheus"
			variable["regex"] = ""
		}

		list[i] = variable
	}

	return map[string]interface{}{"list": list}
}

// normalizeColor converts color names to Grafana color codes
func (s *GrafanaService) normalizeColor(color string) string {
	colorMap := map[string]string{
		"green":       "green",
		"yellow":      "yellow",
		"orange":      "orange",
		"red":         "red",
		"blue":        "blue",
		"purple":      "purple",
		"super-light-green": "super-light-green",
		"light-green": "light-green",
		"semi-dark-green": "semi-dark-green",
		"dark-green":  "dark-green",
		"super-light-yellow": "super-light-yellow",
		"light-yellow": "light-yellow",
		"semi-dark-yellow": "semi-dark-yellow",
		"dark-yellow": "dark-yellow",
		"super-light-orange": "super-light-orange",
		"light-orange": "light-orange",
		"semi-dark-orange": "semi-dark-orange",
		"dark-orange": "dark-orange",
		"super-light-red": "super-light-red",
		"light-red":   "light-red",
		"semi-dark-red": "semi-dark-red",
		"dark-red":    "dark-red",
	}

	if normalized, ok := colorMap[strings.ToLower(color)]; ok {
		return normalized
	}

	// If it starts with #, assume it's a hex color
	if strings.HasPrefix(color, "#") {
		return color
	}

	// Default to green
	return "green"
}
