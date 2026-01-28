package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Limits to protect datasources from overload
const (
	maxMetricsResults     = 500  // Max metrics to return from Prometheus
	maxLabelsResults      = 100  // Max labels to return from Loki
	maxLabelValuesResults = 200  // Max label values to return
	maxTempoResults       = 100  // Max tags/services from Tempo
	metricsCacheTTL       = 5 * time.Minute // Cache TTL for metrics list
)

// metricsCache stores cached metrics to avoid repeated expensive queries
type metricsCache struct {
	sync.RWMutex
	data      map[string]cachedMetrics
}

type cachedMetrics struct {
	metrics   []string
	timestamp time.Time
}

var globalMetricsCache = &metricsCache{
	data: make(map[string]cachedMetrics),
}

func (c *metricsCache) get(key string) ([]string, bool) {
	c.RLock()
	defer c.RUnlock()

	cached, ok := c.data[key]
	if !ok {
		return nil, false
	}

	if time.Since(cached.timestamp) > metricsCacheTTL {
		return nil, false
	}

	return cached.metrics, true
}

func (c *metricsCache) set(key string, metrics []string) {
	c.Lock()
	defer c.Unlock()

	c.data[key] = cachedMetrics{
		metrics:   metrics,
		timestamp: time.Now(),
	}
}

// DiscoveryService handles discovery of Grafana resources
type DiscoveryService struct {
	httpClient   *http.Client
	grafanaURL   string
	grafanaToken string
}

// DatasourceInfo represents a Grafana datasource
type DatasourceInfo struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
}

// UserInfo represents user information
type UserInfo struct {
	Login          string    `json:"login"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	IsGrafanaAdmin bool      `json:"isGrafanaAdmin"`
	Organizations  []OrgInfo `json:"organizations"`
}

// OrgInfo represents organization information
type OrgInfo struct {
	OrgID int64  `json:"orgId"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// EnvironmentContext represents the full Grafana environment context
type EnvironmentContext struct {
	Datasources       []DatasourceInfo `json:"datasources"`
	PrometheusMetrics []string         `json:"prometheusMetrics,omitempty"`
	LokiLabels        []string         `json:"lokiLabels,omitempty"`
	TempoTags         []string         `json:"tempoTags,omitempty"`
	TempoServices     []string         `json:"tempoServices,omitempty"`
	UserInfo          *UserInfo        `json:"userInfo,omitempty"`
	// Default datasources configured in plugin settings
	DefaultPrometheus *DatasourceInfo `json:"defaultPrometheus,omitempty"`
	DefaultLoki       *DatasourceInfo `json:"defaultLoki,omitempty"`
	DefaultTempo      *DatasourceInfo `json:"defaultTempo,omitempty"`
	// Other datasources of same type (not default)
	OtherPrometheus []DatasourceInfo `json:"otherPrometheus,omitempty"`
	OtherLoki       []DatasourceInfo `json:"otherLoki,omitempty"`
	OtherTempo      []DatasourceInfo `json:"otherTempo,omitempty"`
}

// MetricsResponse represents Prometheus label values response
type MetricsResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// NewDiscoveryService creates a new DiscoveryService instance
func NewDiscoveryService(grafanaURL, token string) *DiscoveryService {
	return &DiscoveryService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		grafanaURL:   strings.TrimRight(grafanaURL, "/"),
		grafanaToken: token,
	}
}

// makeRequest performs an HTTP request to the Grafana API
func (s *DiscoveryService) makeRequest(method, path string) ([]byte, error) {
	url := s.grafanaURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+s.grafanaToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	return body, nil
}

// GetDatasources retrieves all configured datasources
func (s *DiscoveryService) GetDatasources() ([]DatasourceInfo, error) {
	body, err := s.makeRequest("GET", "/api/datasources")
	if err != nil {
		log.DefaultLogger.Error("Failed to get datasources", "error", err)
		return nil, err
	}

	var datasources []struct {
		UID       string `json:"uid"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		IsDefault bool   `json:"isDefault"`
	}

	if err := json.Unmarshal(body, &datasources); err != nil {
		return nil, err
	}

	result := make([]DatasourceInfo, len(datasources))
	for i, ds := range datasources {
		result[i] = DatasourceInfo{
			UID:       ds.UID,
			Name:      ds.Name,
			Type:      ds.Type,
			IsDefault: ds.IsDefault,
		}
	}

	log.DefaultLogger.Info("Retrieved datasources", "count", len(result))
	return result, nil
}

// GetPrometheusMetrics retrieves available metrics from a Prometheus datasource
// Uses caching to avoid repeated expensive queries
func (s *DiscoveryService) GetPrometheusMetrics(datasourceUID string) ([]string, error) {
	cacheKey := "prometheus_metrics_" + datasourceUID

	// Check cache first
	if cached, ok := globalMetricsCache.get(cacheKey); ok {
		log.DefaultLogger.Info("Using cached Prometheus metrics", "datasource", datasourceUID, "count", len(cached))
		return cached, nil
	}

	path := fmt.Sprintf("/api/datasources/proxy/uid/%s/api/v1/label/__name__/values", datasourceUID)
	body, err := s.makeRequest("GET", path)
	if err != nil {
		log.DefaultLogger.Error("Failed to get Prometheus metrics", "datasource", datasourceUID, "error", err)
		return nil, err
	}

	var response MetricsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("Prometheus API returned status: %s", response.Status)
	}

	metrics := response.Data

	// Limit results to protect against huge metric counts
	if len(metrics) > maxMetricsResults {
		log.DefaultLogger.Warn("Prometheus metrics truncated", "total", len(metrics), "limit", maxMetricsResults)
		metrics = metrics[:maxMetricsResults]
	}

	// Cache the results
	globalMetricsCache.set(cacheKey, metrics)

	log.DefaultLogger.Info("Retrieved Prometheus metrics", "count", len(metrics))
	return metrics, nil
}

// SearchPrometheusMetrics searches for metrics matching a pattern
func (s *DiscoveryService) SearchPrometheusMetrics(datasourceUID, pattern string) ([]string, error) {
	allMetrics, err := s.GetPrometheusMetrics(datasourceUID)
	if err != nil {
		return nil, err
	}

	pattern = strings.ToLower(pattern)
	var matched []string
	for _, metric := range allMetrics {
		if strings.Contains(strings.ToLower(metric), pattern) {
			matched = append(matched, metric)
		}
	}

	// Limit results to avoid huge responses
	if len(matched) > 100 {
		matched = matched[:100]
	}

	return matched, nil
}

// GetLokiLabels retrieves available labels from a Loki datasource
func (s *DiscoveryService) GetLokiLabels(datasourceUID string) ([]string, error) {
	path := fmt.Sprintf("/api/datasources/proxy/uid/%s/loki/api/v1/labels", datasourceUID)
	body, err := s.makeRequest("GET", path)
	if err != nil {
		log.DefaultLogger.Error("Failed to get Loki labels", "datasource", datasourceUID, "error", err)
		return nil, err
	}

	var response MetricsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("Loki API returned status: %s", response.Status)
	}

	labels := response.Data
	// Limit results to protect against high cardinality
	if len(labels) > maxLabelsResults {
		log.DefaultLogger.Warn("Loki labels truncated", "total", len(labels), "limit", maxLabelsResults)
		labels = labels[:maxLabelsResults]
	}

	log.DefaultLogger.Info("Retrieved Loki labels", "count", len(labels))
	return labels, nil
}

// GetLokiLabelValues retrieves values for a specific Loki label
func (s *DiscoveryService) GetLokiLabelValues(datasourceUID, label string) ([]string, error) {
	path := fmt.Sprintf("/api/datasources/proxy/uid/%s/loki/api/v1/label/%s/values", datasourceUID, label)
	body, err := s.makeRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var response MetricsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	values := response.Data
	// Limit results to protect against high cardinality labels
	if len(values) > maxLabelValuesResults {
		log.DefaultLogger.Warn("Loki label values truncated", "label", label, "total", len(values), "limit", maxLabelValuesResults)
		values = values[:maxLabelValuesResults]
	}

	return values, nil
}

// TempoTagsResponse represents Tempo tags response
type TempoTagsResponse struct {
	TagNames []string `json:"tagNames"`
}

// TempoServicesResponse represents Tempo services response
type TempoServicesResponse struct {
	Data []struct {
		Name string `json:"name"`
	} `json:"data"`
}

// GetTempoTags retrieves available tags from a Tempo datasource
func (s *DiscoveryService) GetTempoTags(datasourceUID string) ([]string, error) {
	path := fmt.Sprintf("/api/datasources/proxy/uid/%s/api/search/tags", datasourceUID)
	body, err := s.makeRequest("GET", path)
	if err != nil {
		log.DefaultLogger.Error("Failed to get Tempo tags", "datasource", datasourceUID, "error", err)
		return nil, err
	}

	var response TempoTagsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	tags := response.TagNames
	// Limit results
	if len(tags) > maxTempoResults {
		log.DefaultLogger.Warn("Tempo tags truncated", "total", len(tags), "limit", maxTempoResults)
		tags = tags[:maxTempoResults]
	}

	log.DefaultLogger.Info("Retrieved Tempo tags", "count", len(tags))
	return tags, nil
}

// GetTempoServices retrieves services from Tempo
func (s *DiscoveryService) GetTempoServices(datasourceUID string) ([]string, error) {
	path := fmt.Sprintf("/api/datasources/proxy/uid/%s/api/search/tag/service.name/values", datasourceUID)
	body, err := s.makeRequest("GET", path)
	if err != nil {
		log.DefaultLogger.Warn("Failed to get Tempo services", "datasource", datasourceUID, "error", err)
		return nil, err
	}

	// Try to parse as array of strings first
	var values []string
	if err := json.Unmarshal(body, &values); err == nil {
		// Limit results
		if len(values) > maxTempoResults {
			log.DefaultLogger.Warn("Tempo services truncated", "total", len(values), "limit", maxTempoResults)
			values = values[:maxTempoResults]
		}
		return values, nil
	}

	// Try alternative format
	var response struct {
		TagValues []string `json:"tagValues"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	services := response.TagValues
	// Limit results
	if len(services) > maxTempoResults {
		log.DefaultLogger.Warn("Tempo services truncated", "total", len(services), "limit", maxTempoResults)
		services = services[:maxTempoResults]
	}

	return services, nil
}

// GetUserInfo retrieves current user information
func (s *DiscoveryService) GetUserInfo() (*UserInfo, error) {
	// Get user details
	body, err := s.makeRequest("GET", "/api/user")
	if err != nil {
		log.DefaultLogger.Error("Failed to get user info", "error", err)
		return nil, err
	}

	var userResp struct {
		Login          string `json:"login"`
		Name           string `json:"name"`
		Email          string `json:"email"`
		IsGrafanaAdmin bool   `json:"isGrafanaAdmin"`
	}

	if err := json.Unmarshal(body, &userResp); err != nil {
		return nil, err
	}

	userInfo := &UserInfo{
		Login:          userResp.Login,
		Name:           userResp.Name,
		Email:          userResp.Email,
		IsGrafanaAdmin: userResp.IsGrafanaAdmin,
	}

	// Get user organizations
	orgsBody, err := s.makeRequest("GET", "/api/user/orgs")
	if err != nil {
		log.DefaultLogger.Warn("Failed to get user orgs", "error", err)
	} else {
		var orgs []OrgInfo
		if err := json.Unmarshal(orgsBody, &orgs); err == nil {
			userInfo.Organizations = orgs
			// Set role from first org (current org)
			if len(orgs) > 0 {
				userInfo.Role = orgs[0].Role
			}
		}
	}

	log.DefaultLogger.Info("Retrieved user info", "login", userInfo.Login)
	return userInfo, nil
}

// GetBasicContext retrieves basic context (datasources only) for initial AI request
func (s *DiscoveryService) GetBasicContext() (*EnvironmentContext, error) {
	ctx := &EnvironmentContext{}

	// Get datasources (essential)
	datasources, err := s.GetDatasources()
	if err != nil {
		log.DefaultLogger.Warn("Failed to get datasources", "error", err)
	} else {
		ctx.Datasources = datasources
	}

	return ctx, nil
}

// GetFullContext retrieves the complete environment context
func (s *DiscoveryService) GetFullContext() (*EnvironmentContext, error) {
	ctx := &EnvironmentContext{}

	// Get datasources
	datasources, err := s.GetDatasources()
	if err != nil {
		log.DefaultLogger.Warn("Failed to get datasources", "error", err)
	} else {
		ctx.Datasources = datasources
	}

	// Get user info
	userInfo, err := s.GetUserInfo()
	if err != nil {
		log.DefaultLogger.Warn("Failed to get user info", "error", err)
	} else {
		ctx.UserInfo = userInfo
	}

	// For each datasource type, get relevant data
	for _, ds := range datasources {
		switch ds.Type {
		case "prometheus":
			// Don't fetch all metrics by default - it can be huge
			// The AI will use search_metrics tool when needed
			break
		case "loki":
			labels, err := s.GetLokiLabels(ds.UID)
			if err == nil {
				ctx.LokiLabels = labels
			}
		case "tempo":
			tags, err := s.GetTempoTags(ds.UID)
			if err == nil {
				ctx.TempoTags = tags
			}
			services, err := s.GetTempoServices(ds.UID)
			if err == nil {
				ctx.TempoServices = services
			}
		}
	}

	return ctx, nil
}

// FindDatasourceByType finds the first datasource of a given type
func (s *DiscoveryService) FindDatasourceByType(dsType string) (*DatasourceInfo, error) {
	datasources, err := s.GetDatasources()
	if err != nil {
		return nil, err
	}

	for _, ds := range datasources {
		if ds.Type == dsType {
			return &ds, nil
		}
	}

	return nil, fmt.Errorf("no datasource of type %s found", dsType)
}

// SearchUser searches for a user by login or email within the current organization
func (s *DiscoveryService) SearchUser(loginOrEmail string) (*UserInfo, error) {
	// Use org users API which works with org admin permissions
	body, err := s.makeRequest("GET", "/api/org/users")
	if err != nil {
		log.DefaultLogger.Error("Failed to get org users", "error", err)
		return nil, err
	}

	var orgUsers []struct {
		UserID         int64  `json:"userId"`
		OrgID          int64  `json:"orgId"`
		Login          string `json:"login"`
		Name           string `json:"name"`
		Email          string `json:"email"`
		Role           string `json:"role"`
		IsGrafanaAdmin bool   `json:"isGrafanaAdmin"`
	}

	if err := json.Unmarshal(body, &orgUsers); err != nil {
		return nil, err
	}

	// Search for user by email or login (case insensitive)
	loginOrEmailLower := strings.ToLower(loginOrEmail)
	for _, user := range orgUsers {
		if strings.ToLower(user.Email) == loginOrEmailLower || strings.ToLower(user.Login) == loginOrEmailLower {
			userInfo := &UserInfo{
				Login:          user.Login,
				Name:           user.Name,
				Email:          user.Email,
				Role:           user.Role,
				IsGrafanaAdmin: user.IsGrafanaAdmin,
				Organizations: []OrgInfo{
					{
						OrgID: user.OrgID,
						Role:  user.Role,
					},
				},
			}
			log.DefaultLogger.Info("Found user", "login", userInfo.Login, "role", userInfo.Role)
			return userInfo, nil
		}
	}

	return nil, fmt.Errorf("usuário '%s' não encontrado na organização atual", loginOrEmail)
}

// DashboardInfo represents a dashboard search result
type DashboardSearchResult struct {
	UID       string   `json:"uid"`
	Title     string   `json:"title"`
	FolderUID string   `json:"folderUid,omitempty"`
	Folder    string   `json:"folderTitle,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Type      string   `json:"type"`
}

// SearchDashboards searches for dashboards by query string
// Uses flexible keyword matching - a dashboard matches if it contains ANY of the keywords
// Results are limited to maxDashboardResults to avoid overloading responses
const maxDashboardResults = 100

func (s *DiscoveryService) SearchDashboards(query string) ([]DashboardSearchResult, error) {
	// Always get all dashboards and filter client-side for better matching
	searchURL := "/api/search?type=dash-db&limit=1000"

	body, err := s.makeRequest("GET", searchURL)
	if err != nil {
		log.DefaultLogger.Error("Failed to search dashboards", "error", err)
		return nil, err
	}

	var results []struct {
		UID         string   `json:"uid"`
		Title       string   `json:"title"`
		FolderUID   string   `json:"folderUid"`
		FolderTitle string   `json:"folderTitle"`
		Tags        []string `json:"tags"`
		Type        string   `json:"type"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return nil, err
	}

	// If no query, return all dashboards (limited)
	if query == "" {
		totalCount := len(results)
		if totalCount > maxDashboardResults {
			results = results[:maxDashboardResults]
		}
		dashboards := make([]DashboardSearchResult, 0, len(results))
		for _, r := range results {
			dashboards = append(dashboards, DashboardSearchResult{
				UID:       r.UID,
				Title:     r.Title,
				FolderUID: r.FolderUID,
				Folder:    r.FolderTitle,
				Tags:      r.Tags,
				Type:      r.Type,
			})
		}
		if totalCount > maxDashboardResults {
			log.DefaultLogger.Info("Found dashboards (all, truncated)", "returned", len(dashboards), "total", totalCount)
		} else {
			log.DefaultLogger.Info("Found dashboards (all)", "count", len(dashboards))
		}
		return dashboards, nil
	}

	// Extract keywords from query (split by spaces, ignore common words)
	keywords := extractKeywords(query)
	log.DefaultLogger.Info("Searching dashboards", "query", query, "keywords", keywords)

	// Filter dashboards that match any keyword
	dashboards := make([]DashboardSearchResult, 0)
	for _, r := range results {
		titleLower := strings.ToLower(r.Title)
		folderLower := strings.ToLower(r.FolderTitle)
		tagsLower := strings.ToLower(strings.Join(r.Tags, " "))

		// Check if any keyword matches
		matched := false
		for _, keyword := range keywords {
			if strings.Contains(titleLower, keyword) ||
				strings.Contains(folderLower, keyword) ||
				strings.Contains(tagsLower, keyword) {
				matched = true
				break
			}
		}

		if matched {
			dashboards = append(dashboards, DashboardSearchResult{
				UID:       r.UID,
				Title:     r.Title,
				FolderUID: r.FolderUID,
				Folder:    r.FolderTitle,
				Tags:      r.Tags,
				Type:      r.Type,
			})
			// Limit results
			if len(dashboards) >= maxDashboardResults {
				log.DefaultLogger.Info("Found dashboards (truncated)", "returned", len(dashboards), "query", query)
				return dashboards, nil
			}
		}
	}

	log.DefaultLogger.Info("Found dashboards", "count", len(dashboards), "query", query)
	return dashboards, nil
}

// extractKeywords extracts meaningful keywords from a search query
func extractKeywords(query string) []string {
	// Common words to ignore (Portuguese and English)
	stopWords := map[string]bool{
		// Portuguese
		"de": true, "do": true, "da": true, "dos": true, "das": true,
		"um": true, "uma": true, "uns": true, "umas": true,
		"o": true, "a": true, "os": true, "as": true,
		"e": true, "ou": true, "para": true, "com": true, "sem": true,
		"em": true, "no": true, "na": true, "nos": true, "nas": true,
		"por": true, "pelo": true, "pela": true,
		"que": true, "qual": true, "quais": true,
		"existe": true, "existem": true, "algum": true, "alguma": true,
		"sobre": true, "como": true, "este": true, "esta": true,
		// English
		"the": true, "an": true, "of": true, "for": true,
		"and": true, "or": true, "with": true, "without": true,
		"in": true, "on": true, "at": true, "to": true, "from": true,
		"is": true, "are": true, "was": true, "were": true,
		"any": true, "some": true, "there": true,
	}

	// Split query into words
	words := strings.Fields(strings.ToLower(query))

	// Filter out stop words and short words
	keywords := make([]string, 0)
	for _, word := range words {
		// Remove punctuation
		word = strings.Trim(word, ".,;:!?\"'()[]{}")

		// Skip stop words and very short words
		if len(word) < 3 || stopWords[word] {
			continue
		}
		keywords = append(keywords, word)
	}

	return keywords
}

// ListAllDashboards lists all dashboards in the instance
func (s *DiscoveryService) ListAllDashboards() ([]DashboardSearchResult, error) {
	return s.SearchDashboards("")
}

// DashboardExists checks if a dashboard with the given title already exists
func (s *DiscoveryService) DashboardExists(title string) (bool, string) {
	// Search for dashboards with matching title
	searchURL := fmt.Sprintf("/api/search?query=%s&type=dash-db", url.QueryEscape(title))
	body, err := s.makeRequest("GET", searchURL)
	if err != nil {
		log.DefaultLogger.Warn("Failed to search dashboards", "error", err)
		return false, ""
	}

	var results []struct {
		UID   string `json:"uid"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return false, ""
	}

	// Check for exact title match (case insensitive)
	titleLower := strings.ToLower(title)
	for _, dash := range results {
		if strings.ToLower(dash.Title) == titleLower {
			log.DefaultLogger.Info("Dashboard already exists", "title", title, "uid", dash.UID)
			return true, dash.UID
		}
	}

	return false, ""
}

// FolderExists checks if a folder with the given name already exists
func (s *DiscoveryService) FolderExists(name string) (bool, int64) {
	body, err := s.makeRequest("GET", "/api/folders")
	if err != nil {
		return false, 0
	}

	var folders []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}

	if err := json.Unmarshal(body, &folders); err != nil {
		return false, 0
	}

	nameLower := strings.ToLower(name)
	for _, folder := range folders {
		if strings.ToLower(folder.Title) == nameLower {
			return true, folder.ID
		}
	}

	return false, 0
}

// FindDefaultDatasource finds the default datasource
func (s *DiscoveryService) FindDefaultDatasource() (*DatasourceInfo, error) {
	datasources, err := s.GetDatasources()
	if err != nil {
		return nil, err
	}

	for _, ds := range datasources {
		if ds.IsDefault {
			return &ds, nil
		}
	}

	// If no default, return first prometheus
	for _, ds := range datasources {
		if ds.Type == "prometheus" {
			return &ds, nil
		}
	}

	// Return first datasource
	if len(datasources) > 0 {
		return &datasources[0], nil
	}

	return nil, fmt.Errorf("no datasources configured")
}

// DashboardWithPermission represents a dashboard with permission info
type DashboardWithPermission struct {
	UID        string   `json:"uid"`
	Title      string   `json:"title"`
	Folder     string   `json:"folder"`
	FolderUID  string   `json:"folderUid,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	URL        string   `json:"url"`
	CreatedBy  string   `json:"createdBy"`
	UpdatedBy  string   `json:"updatedBy"`
	Permission string   `json:"permission"` // "owner", "edit", "view"
}

// DashboardPermissionItem represents a permission entry for a dashboard
type DashboardPermissionItem struct {
	Role       string `json:"role"`
	Permission int    `json:"permission"` // 1=View, 2=Edit, 4=Admin
	UserLogin  string `json:"userLogin,omitempty"`
	UserEmail  string `json:"userEmail,omitempty"`
	TeamID     int64  `json:"teamId,omitempty"`
}

// GetDashboardDetails gets detailed information about a dashboard including creator
func (s *DiscoveryService) GetDashboardDetails(uid string) (*DashboardWithPermission, error) {
	path := fmt.Sprintf("/api/dashboards/uid/%s", uid)
	body, err := s.makeRequest("GET", path)
	if err != nil {
		return nil, err
	}

	var response struct {
		Dashboard struct {
			UID   string   `json:"uid"`
			Title string   `json:"title"`
			Tags  []string `json:"tags"`
		} `json:"dashboard"`
		Meta struct {
			FolderTitle string `json:"folderTitle"`
			FolderUID   string `json:"folderUid"`
			URL         string `json:"url"`
			CreatedBy   string `json:"createdBy"`
			UpdatedBy   string `json:"updatedBy"`
			CanEdit     bool   `json:"canEdit"`
			CanAdmin    bool   `json:"canAdmin"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	permission := "view"
	if response.Meta.CanAdmin {
		permission = "admin"
	} else if response.Meta.CanEdit {
		permission = "edit"
	}

	return &DashboardWithPermission{
		UID:        response.Dashboard.UID,
		Title:      response.Dashboard.Title,
		Folder:     response.Meta.FolderTitle,
		FolderUID:  response.Meta.FolderUID,
		Tags:       response.Dashboard.Tags,
		URL:        response.Meta.URL,
		CreatedBy:  response.Meta.CreatedBy,
		UpdatedBy:  response.Meta.UpdatedBy,
		Permission: permission,
	}, nil
}

// GetUserDashboards gets dashboards where the user is owner or has edit permission
// userLogin, userEmail and userRole are used to identify and check permissions for the user
func (s *DiscoveryService) GetUserDashboards(userLogin, userEmail, userRole string, permissionFilter string) ([]DashboardWithPermission, error) {
	// First, get all dashboards
	searchURL := "/api/search?type=dash-db&limit=500"
	body, err := s.makeRequest("GET", searchURL)
	if err != nil {
		return nil, err
	}

	var searchResults []struct {
		UID         string   `json:"uid"`
		Title       string   `json:"title"`
		FolderTitle string   `json:"folderTitle"`
		FolderUID   string   `json:"folderUid"`
		Tags        []string `json:"tags"`
		URL         string   `json:"url"`
	}

	if err := json.Unmarshal(body, &searchResults); err != nil {
		return nil, err
	}

	userLoginLower := strings.ToLower(userLogin)
	userEmailLower := strings.ToLower(userEmail)
	userRoleLower := strings.ToLower(userRole)

	// Build the tag pattern to match dashboards requested by this user via AI
	// Tag format: "solicitado-por:email@example.com"
	userRequestedTag := ""
	if userEmailLower != "" {
		userRequestedTag = "solicitado-por:" + userEmailLower
	}

	log.DefaultLogger.Info("Checking user dashboards", "login", userLogin, "email", userEmail, "role", userRole, "filter", permissionFilter, "requestedTag", userRequestedTag)

	var result []DashboardWithPermission

	// For each dashboard, get details to check ownership and permissions
	for _, dash := range searchResults {
		details, err := s.GetDashboardDetails(dash.UID)
		if err != nil {
			log.DefaultLogger.Warn("Failed to get dashboard details", "uid", dash.UID, "error", err)
			continue
		}

		// Check if user is owner:
		// 1. Created the dashboard directly (createdBy matches)
		// 2. Requested the dashboard via AI (has tag "solicitado-por:email")
		isOwner := false
		isAIRequested := false

		// Check createdBy field
		createdByLower := strings.ToLower(details.CreatedBy)
		if userLoginLower != "" && createdByLower == userLoginLower {
			isOwner = true
		} else if userEmailLower != "" && createdByLower == userEmailLower {
			isOwner = true
		}

		// Check for AI-requested tag (dashboards created via chat)
		if userRequestedTag != "" && len(details.Tags) > 0 {
			for _, tag := range details.Tags {
				if strings.ToLower(tag) == userRequestedTag {
					isAIRequested = true
					isOwner = true // User who requested via AI is considered owner
					break
				}
			}
		}

		if isOwner {
			if isAIRequested {
				details.Permission = "owner (via IA)"
			} else {
				details.Permission = "owner"
			}
		}

		// Determine if user can edit based on their role
		// Note: The API returns permissions from Service Account's perspective
		// We need to interpret based on user's actual role
		canEdit := false
		switch userRoleLower {
		case "admin":
			// Admins can edit everything
			canEdit = true
		case "editor":
			// Editors can edit dashboards (unless restricted by folder permissions)
			// Since we can't easily check folder-level permissions for the user,
			// we assume editors can edit dashboards that are editable
			canEdit = true
		case "viewer":
			// Viewers can only edit dashboards they own or where they have explicit permission
			// Since we can't check ACLs from user perspective, only allow owned dashboards
			canEdit = isOwner
		default:
			// Unknown role - be restrictive
			canEdit = isOwner
		}

		// Filter based on permission request
		include := false
		switch permissionFilter {
		case "owner":
			// Only dashboards created/requested by the user
			include = isOwner
		case "edit":
			// Dashboards the user can edit (based on role)
			include = canEdit
		case "all":
			// All visible dashboards
			include = true
		default:
			include = canEdit
		}

		if include {
			// Set permission label for display
			if isOwner {
				if isAIRequested {
					details.Permission = "owner (via IA)"
				} else {
					details.Permission = "owner"
				}
			} else if canEdit {
				details.Permission = "edit"
			} else {
				details.Permission = "view"
			}
			result = append(result, *details)
		}

		// Limit results to avoid performance issues
		if len(result) >= 50 {
			log.DefaultLogger.Info("User dashboards truncated", "returned", len(result))
			break
		}
	}

	log.DefaultLogger.Info("Found user dashboards", "count", len(result), "filter", permissionFilter, "user", userLogin, "role", userRole)
	return result, nil
}
