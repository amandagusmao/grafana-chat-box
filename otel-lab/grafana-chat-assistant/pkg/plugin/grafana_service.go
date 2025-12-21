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
	url   string
	token string
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

// CreateDashboard creates a new dashboard in Grafana
func (s *GrafanaService) CreateDashboard(data DashboardData) (*DashboardResult, error) {
	log.DefaultLogger.Info("Creating dashboard", "title", data.Title, "datasource", data.Datasource)

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
		"overwrite": true,
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
