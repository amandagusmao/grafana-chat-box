package plugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Message represents a chat message
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// ChatRequest represents the incoming chat request
type ChatRequest struct {
	Messages         []Message              `json:"messages"`
	DashboardContext map[string]interface{} `json:"dashboardContext"`
}

// ChatResponse represents the chat response
type ChatResponse struct {
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Success   bool                   `json:"success"`
	Dashboard *DashboardInfo         `json:"dashboard,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ToolUsed  string                 `json:"toolUsed,omitempty"`
}

// DashboardInfo contains created dashboard information
type DashboardInfo struct {
	URL   string `json:"url"`
	UID   string `json:"uid"`
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": "",
	})
}

// handleListDatasources returns available datasources grouped by type
func handleListDatasources(w http.ResponseWriter, r *http.Request, settings AppSettings) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Method not allowed",
		})
		return
	}

	// SECURITY: Require authenticated user
	loggedUser := getLoggedUser(r)
	if loggedUser.Login == "" && loggedUser.Email == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Autenticação necessária.",
		})
		return
	}

	// Check if Grafana settings are configured
	if settings.GrafanaURL == "" || settings.GrafanaToken == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"prometheus": []DatasourceInfo{},
			"loki":       []DatasourceInfo{},
			"tempo":      []DatasourceInfo{},
			"other":      []DatasourceInfo{},
		})
		return
	}

	discoveryService := NewDiscoveryService(settings.GrafanaURL, settings.GrafanaToken)
	datasources, err := discoveryService.GetDatasources()
	if err != nil {
		log.DefaultLogger.Error("Failed to get datasources", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Não foi possível obter os datasources. Verifique as configurações do plugin.",
		})
		return
	}

	// Group datasources by type
	result := map[string][]DatasourceInfo{
		"prometheus": {},
		"loki":       {},
		"tempo":      {},
		"other":      {},
	}

	for _, ds := range datasources {
		switch ds.Type {
		case "prometheus":
			result["prometheus"] = append(result["prometheus"], ds)
		case "loki":
			result["loki"] = append(result["loki"], ds)
		case "tempo":
			result["tempo"] = append(result["tempo"], ds)
		default:
			result["other"] = append(result["other"], ds)
		}
	}

	json.NewEncoder(w).Encode(result)
}

// LoggedUser represents the currently logged in user from Grafana session
type LoggedUser struct {
	Login string
	Email string
	Role  string
}

// getLoggedUser extracts the logged user from request headers
func getLoggedUser(r *http.Request) *LoggedUser {
	user := &LoggedUser{}

	// Try X-Grafana-Id header (JWT) first
	grafanaID := r.Header.Get("X-Grafana-Id")
	if grafanaID != "" {
		// Decode JWT payload (second part)
		parts := strings.Split(grafanaID, ".")
		if len(parts) >= 2 {
			payload := parts[1]
			// Add padding if necessary
			switch len(payload) % 4 {
			case 2:
				payload += "=="
			case 3:
				payload += "="
			}

			decoded, err := base64.URLEncoding.DecodeString(payload)
			if err != nil {
				decoded, err = base64.StdEncoding.DecodeString(payload)
			}

			if err == nil {
				var claims struct {
					Email    string `json:"email"`
					Username string `json:"username"`
					Login    string `json:"login"`
					Sub      string `json:"sub"`
					Name     string `json:"name"`
					Role     string `json:"role"`
				}
				if err := json.Unmarshal(decoded, &claims); err == nil {
					user.Email = claims.Email
					user.Role = claims.Role
					// Try multiple fields for login
					if claims.Username != "" {
						user.Login = claims.Username
					} else if claims.Login != "" {
						user.Login = claims.Login
					} else if claims.Sub != "" {
						user.Login = claims.Sub
					}
					log.DefaultLogger.Info("Decoded user from JWT",
						"login", user.Login, "email", user.Email, "role", user.Role)
				}
			}
		}
	}

	// Fallback: try standard Grafana plugin headers
	if user.Login == "" {
		if login := r.Header.Get("X-Grafana-User"); login != "" {
			user.Login = login
			log.DefaultLogger.Info("Got user from X-Grafana-User header", "login", login)
		}
	}

	return user
}

// handleChat handles chat requests with full tool support
func handleChat(w http.ResponseWriter, r *http.Request, settings AppSettings, authService *AuthService) {
	w.Header().Set("Content-Type", "application/json")

	// Capture logged user from headers
	loggedUser := getLoggedUser(r)
	log.DefaultLogger.Info("Request from user (from headers)", "login", loggedUser.Login, "email", loggedUser.Email, "role", loggedUser.Role)

	// SECURITY: Require authenticated user for all operations
	if loggedUser.Login == "" && loggedUser.Email == "" {
		log.DefaultLogger.Warn("Unauthenticated request blocked")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Autenticação necessária. Faça login no Grafana para usar o chat.",
		})
		return
	}

	// SECURITY: Verify user role via Grafana API (don't trust headers alone)
	if settings.GrafanaURL != "" && settings.GrafanaToken != "" {
		discoveryForAuth := NewDiscoveryService(settings.GrafanaURL, settings.GrafanaToken)

		// Try to verify by login first, then by email
		lookupKey := loggedUser.Login
		if lookupKey == "" {
			lookupKey = loggedUser.Email
		}

		if lookupKey != "" {
			verified, err := discoveryForAuth.VerifyUserRole(lookupKey)
			if err != nil {
				log.DefaultLogger.Warn("Could not verify user role via API, using header value",
					"user", lookupKey, "headerRole", loggedUser.Role, "error", err)
			} else {
				// Override with verified data from API
				loggedUser.Role = verified.Role
				if loggedUser.Login == "" {
					loggedUser.Login = verified.Login
				}
				if loggedUser.Email == "" {
					loggedUser.Email = verified.Email
				}
				log.DefaultLogger.Info("User role verified via API",
					"login", loggedUser.Login, "email", loggedUser.Email, "role", loggedUser.Role)
			}
		}
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Read request body with size limit to prevent memory exhaustion (100KB max)
	const maxBodySize = 100 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.DefaultLogger.Warn("Failed to read request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Requisição muito grande. O tamanho máximo permitido é 100KB.",
		})
		return
	}
	defer r.Body.Close()

	// Parse request
	var chatReq ChatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		log.DefaultLogger.Error("Failed to parse request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// INPUT VALIDATION: message length and count limits
	const maxMessageLength = 2000
	const maxMessagesInHistory = 30

	if len(chatReq.Messages) > 0 {
		lastMsg := chatReq.Messages[len(chatReq.Messages)-1]
		if len(lastMsg.Content) > maxMessageLength {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ChatResponse{
				Success: false,
				Error:   fmt.Sprintf("Mensagem muito longa. Máximo permitido: %d caracteres.", maxMessageLength),
			})
			return
		}
	}

	// Truncate history to avoid token overflow — keep system-relevant messages
	if len(chatReq.Messages) > maxMessagesInHistory {
		chatReq.Messages = chatReq.Messages[len(chatReq.Messages)-maxMessagesInHistory:]
		log.DefaultLogger.Info("Conversation history truncated",
			"kept", maxMessagesInHistory, "user", loggedUser.Login)
	}

	// Validate authService
	if authService == nil {
		log.DefaultLogger.Error("AuthService not initialized - credentials not configured")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Credenciais não configuradas. Por favor, configure o identificador, senha e endpoint nas configurações do plugin.",
		})
		return
	}

	// Get authentication token (uses cached token if valid)
	token, err := authService.GetToken()
	if err != nil {
		log.DefaultLogger.Error("Failed to get authentication token", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Falha na autenticação com o serviço de IA. Verifique as configurações do plugin.",
		})
		return
	}

	// Create discovery service if Grafana settings are configured
	var discoveryService *DiscoveryService
	var envContext *EnvironmentContext

	if settings.GrafanaURL != "" && settings.GrafanaToken != "" {
		discoveryService = NewDiscoveryService(settings.GrafanaURL, settings.GrafanaToken)

		// Get basic context (datasources) for initial request
		envContext, err = discoveryService.GetBasicContext()
		if err != nil {
			log.DefaultLogger.Warn("Failed to get environment context", "error", err)
		}

		// Enrich context with configured default datasources
		if envContext != nil {
			enrichContextWithDefaults(envContext, settings)
		}
	}

	// Process chat with AI
	aiService := NewAIService(settings.AIEndpoint, settings.AIModel)
	aiResponse, err := aiService.ProcessChatWithContext(chatReq.Messages, token, envContext)
	if err != nil {
		log.DefaultLogger.Error("AI processing failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Error:   "Não foi possível processar sua mensagem. Tente novamente em alguns instantes.",
		})
		return
	}

	// Handle tool calls
	if aiResponse.Type == "function_call" {
		response := handleToolCall(aiResponse, settings, discoveryService, token, aiService, chatReq.Messages, envContext, loggedUser)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Regular chat response
	json.NewEncoder(w).Encode(ChatResponse{
		Type:    "message",
		Message: aiResponse.Response,
		Success: true,
	})
}

// handleToolCall processes AI tool calls and returns appropriate response
func handleToolCall(
	aiResponse *AIResponse,
	settings AppSettings,
	discoveryService *DiscoveryService,
	token string,
	aiService *AIService,
	originalMessages []Message,
	envContext *EnvironmentContext,
	loggedUser *LoggedUser,
) ChatResponse {
	funcName := aiResponse.Function

	log.DefaultLogger.Info("Processing tool call", "function", funcName)

	// Check if Grafana is configured for tools that need it
	grafanaRequired := []string{"search_metrics", "list_datasources", "search_loki_labels", "search_tempo_services", "search_user", "search_dashboards", "get_my_dashboards", "create_dashboard"}
	needsGrafana := false
	for _, f := range grafanaRequired {
		if funcName == f {
			needsGrafana = true
			break
		}
	}

	if needsGrafana && (settings.GrafanaURL == "" || settings.GrafanaToken == "") {
		return ChatResponse{
			Type:    "message",
			Message: "Para usar esta funcionalidade, configure a URL e Token do Grafana nas configurações do plugin.",
			Success: true,
		}
	}

	if needsGrafana && discoveryService == nil {
		discoveryService = NewDiscoveryService(settings.GrafanaURL, settings.GrafanaToken)
	}

	// SECURITY: Check permissions for write operations (whitelist approach)
	writeOperations := []string{"create_dashboard"}
	for _, op := range writeOperations {
		if funcName == op {
			// Only Editor and Admin can create dashboards
			userRole := strings.ToLower(loggedUser.Role)
			if userRole != "editor" && userRole != "admin" {
				log.DefaultLogger.Warn("User without write permission attempted write operation",
					"function", funcName, "user", loggedUser.Login, "role", loggedUser.Role)
				return ChatResponse{
					Type:    "message",
					Message: fmt.Sprintf("Você não tem permissão para criar dashboards. Seu role atual é **%s**. Solicite ao administrador permissões de **Editor** ou **Admin**.", loggedUser.Role),
					Success: true,
				}
			}
			break
		}
	}

	// SECURITY: Validate dashboard parameters before creation
	if funcName == "create_dashboard" {
		validationError := validateDashboardParams(aiResponse.FunctionArgs, discoveryService)
		if validationError != "" {
			return ChatResponse{
				Type:    "message",
				Message: validationError,
				Success: true,
			}
		}
	}

	// Execute the tool and get result
	toolResult, toolError := executeToolCall(funcName, aiResponse.FunctionArgs, discoveryService, settings, loggedUser)

	if toolError != nil {
		log.DefaultLogger.Error("Tool execution failed", "function", funcName, "error", toolError)
		return ChatResponse{
			Type:    "message",
			Message: "Não foi possível completar a operação solicitada. Tente novamente ou reformule seu pedido.",
			Success: true,
		}
	}

	// For create_dashboard, return immediately with the result
	if funcName == "create_dashboard" {
		var dashResult struct {
			Success bool   `json:"success"`
			UID     string `json:"uid"`
			ID      int64  `json:"id"`
			Title   string `json:"title"`
			Error   string `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(toolResult), &dashResult); err == nil && dashResult.Success {
			return ChatResponse{
				Type:     "dashboard_created",
				Message:  fmt.Sprintf("Dashboard \"%s\" criado com sucesso!", dashResult.Title),
				Success:  true,
				ToolUsed: funcName,
				Dashboard: &DashboardInfo{
					URL:   settings.GrafanaURL + "/d/" + dashResult.UID,
					UID:   dashResult.UID,
					ID:    dashResult.ID,
					Title: dashResult.Title,
				},
			}
		} else if dashResult.Error != "" {
			return ChatResponse{
				Type:    "message",
				Message: fmt.Sprintf("Erro ao criar dashboard: %s", dashResult.Error),
				Success: true,
			}
		}
	}

	// For discovery tools, return the result directly so the user can see what was found
	// The AI will use this information in subsequent requests
	switch funcName {
	case "search_metrics":
		var metrics []string
		if err := json.Unmarshal([]byte(toolResult), &metrics); err == nil {
			if len(metrics) == 0 {
				// Buscar todas as métricas para mostrar os prefixos disponíveis
				allMetricsHint := ""
				if discoveryService != nil {
					if promDS, err := discoveryService.FindDatasourceByType("prometheus"); err == nil {
						if allMetrics, err := discoveryService.GetPrometheusMetrics(promDS.UID); err == nil && len(allMetrics) > 0 {
							prefixes := getMetricPrefixes(allMetrics)
							allMetricsHint = fmt.Sprintf("\n\nPrefixos de métricas disponíveis: `%s`\n\nExemplos de métricas:\n%s",
								strings.Join(prefixes, "`, `"),
								formatSampleMetrics(allMetrics, 10))
						}
					}
				}
				return ChatResponse{
					Type:     "message",
					Message:  "Nenhuma métrica encontrada com esse padrão." + allMetricsHint,
					Success:  true,
					ToolUsed: funcName,
				}
			}
			// Format metrics nicely
			metricsDisplay := formatMetricsList(metrics)
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("Encontrei %d métricas:\n\n%s\n\nPosso criar um dashboard com essas métricas?", len(metrics), metricsDisplay),
				Success:  true,
				ToolUsed: funcName,
			}
		}

	case "list_datasources":
		var datasources []DatasourceInfo
		if err := json.Unmarshal([]byte(toolResult), &datasources); err == nil {
			if len(datasources) == 0 {
				return ChatResponse{
					Type:     "message",
					Message:  "Nenhum datasource configurado no Grafana.",
					Success:  true,
					ToolUsed: funcName,
				}
			}
			dsDisplay := formatDatasourcesList(datasources)
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("Datasources disponíveis:\n\n%s", dsDisplay),
				Success:  true,
				ToolUsed: funcName,
			}
		}

	case "search_loki_labels":
		var labels []string
		if err := json.Unmarshal([]byte(toolResult), &labels); err == nil {
			if len(labels) == 0 {
				return ChatResponse{
					Type:     "message",
					Message:  "Nenhum label Loki encontrado. Verifique se há logs sendo enviados.",
					Success:  true,
					ToolUsed: funcName,
				}
			}
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("Labels Loki disponíveis:\n\n`%s`", strings.Join(labels, "`, `")),
				Success:  true,
				ToolUsed: funcName,
			}
		}

	case "search_tempo_services":
		var services []string
		if err := json.Unmarshal([]byte(toolResult), &services); err == nil {
			if len(services) == 0 {
				return ChatResponse{
					Type:     "message",
					Message:  "Nenhum serviço Tempo encontrado. Verifique se há traces sendo enviados.",
					Success:  true,
					ToolUsed: funcName,
				}
			}
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("Serviços com traces:\n\n`%s`", strings.Join(services, "`, `")),
				Success:  true,
				ToolUsed: funcName,
			}
		}

	case "search_dashboards":
		var dashboards []DashboardSearchResult
		if err := json.Unmarshal([]byte(toolResult), &dashboards); err == nil {
			if len(dashboards) == 0 {
				return ChatResponse{
					Type:     "message",
					Message:  "Nenhum dashboard encontrado com esse critério de busca.",
					Success:  true,
					ToolUsed: funcName,
				}
			}
			dashDisplay := formatDashboardsList(dashboards)
			// Check if results might be truncated (limit is 100)
			truncatedMsg := ""
			if len(dashboards) >= 100 {
				truncatedMsg = "\n\n*Nota: Exibindo apenas os primeiros 100 resultados. Refine sua busca para resultados mais específicos.*"
			}
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("Encontrei **%d dashboard(s)**:\n\n%s%s", len(dashboards), dashDisplay, truncatedMsg),
				Success:  true,
				ToolUsed: funcName,
			}
		}

	case "get_my_dashboards":
		var dashboards []DashboardWithPermission
		if err := json.Unmarshal([]byte(toolResult), &dashboards); err == nil {
			if len(dashboards) == 0 {
				// Check filter to give appropriate message
				var params struct {
					Filter string `json:"filter"`
				}
				json.Unmarshal(aiResponse.FunctionArgs, &params)

				filterMsg := "com permissão de edição"
				if params.Filter == "owner" {
					filterMsg = "que você criou"
				}
				return ChatResponse{
					Type:     "message",
					Message:  fmt.Sprintf("Nenhum dashboard encontrado %s.", filterMsg),
					Success:  true,
					ToolUsed: funcName,
				}
			}
			dashDisplay := formatUserDashboardsList(dashboards)
			truncatedMsg := ""
			if len(dashboards) >= 50 {
				truncatedMsg = "\n\n*Nota: Exibindo apenas os primeiros 50 resultados.*"
			}
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("Encontrei **%d dashboard(s)**:\n\n%s%s", len(dashboards), dashDisplay, truncatedMsg),
				Success:  true,
				ToolUsed: funcName,
			}
		}

	case "search_user":
		// Check if user is asking about themselves
		var params struct {
			LoginOrEmail string `json:"login_or_email"`
		}
		json.Unmarshal(aiResponse.FunctionArgs, &params)

		requestedUser := strings.ToLower(params.LoginOrEmail)
		currentLogin := strings.ToLower(loggedUser.Login)
		currentEmail := strings.ToLower(loggedUser.Email)

		// Allow if asking about themselves (by login or email)
		isOwnUser := (currentLogin != "" && requestedUser == currentLogin) ||
			(currentEmail != "" && requestedUser == currentEmail)

		log.DefaultLogger.Info("User permission check",
			"requestedUser", requestedUser,
			"currentLogin", currentLogin,
			"currentEmail", currentEmail,
			"isOwnUser", isOwnUser,
			"role", loggedUser.Role)

		// Block if no authenticated user (API call without valid session)
		if currentLogin == "" && currentEmail == "" {
			return ChatResponse{
				Type:     "message",
				Message:  "Não foi possível identificar o usuário autenticado. Esta funcionalidade requer autenticação válida no Grafana.",
				Success:  true,
				ToolUsed: funcName,
			}
		}

		if !isOwnUser && strings.ToLower(loggedUser.Role) != "admin" {
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("Por questões de privacidade, você só pode consultar informações do seu próprio usuário (%s). Para consultar outros usuários, é necessário ter permissão de Admin.", loggedUser.Email),
				Success:  true,
				ToolUsed: funcName,
			}
		}

		var userInfo UserInfo
		if err := json.Unmarshal([]byte(toolResult), &userInfo); err == nil {
			adminStr := "Não"
			if userInfo.IsGrafanaAdmin {
				adminStr = "Sim"
			}
			orgsInfo := ""
			if len(userInfo.Organizations) > 0 {
				orgsInfo = "\n\n**Organizações:**\n"
				for _, org := range userInfo.Organizations {
					orgsInfo += fmt.Sprintf("- %s (Role: %s)\n", org.Name, org.Role)
				}
			}
			return ChatResponse{
				Type:     "message",
				Message:  fmt.Sprintf("**Usuário encontrado:**\n- Login: %s\n- Nome: %s\n- Email: %s\n- Role: **%s**\n- Admin do Grafana: %s%s", userInfo.Login, userInfo.Name, userInfo.Email, userInfo.Role, adminStr, orgsInfo),
				Success:  true,
				ToolUsed: funcName,
			}
		}
	}

	// Default: return the raw tool result
	return ChatResponse{
		Type:     "message",
		Message:  toolResult,
		Success:  true,
		ToolUsed: funcName,
	}
}

// executeToolCall executes a specific tool and returns the result
func executeToolCall(
	funcName string,
	args json.RawMessage,
	discoveryService *DiscoveryService,
	settings AppSettings,
	loggedUser *LoggedUser,
) (string, error) {
	switch funcName {
	case "search_metrics":
		var params struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid parameters: %w", err)
		}

		// Find Prometheus datasource
		promDS, err := discoveryService.FindDatasourceByType("prometheus")
		if err != nil {
			return "", fmt.Errorf("no Prometheus datasource found: %w", err)
		}

		metrics, err := discoveryService.SearchPrometheusMetrics(promDS.UID, params.Pattern)
		if err != nil {
			return "", fmt.Errorf("failed to search metrics: %w", err)
		}

		result, _ := json.Marshal(metrics)
		return string(result), nil

	case "list_datasources":
		datasources, err := discoveryService.GetDatasources()
		if err != nil {
			return "", fmt.Errorf("failed to get datasources: %w", err)
		}

		result, _ := json.Marshal(datasources)
		return string(result), nil

	case "search_loki_labels":
		lokiDS, err := discoveryService.FindDatasourceByType("loki")
		if err != nil {
			return "", fmt.Errorf("no Loki datasource found: %w", err)
		}

		labels, err := discoveryService.GetLokiLabels(lokiDS.UID)
		if err != nil {
			return "", fmt.Errorf("failed to get Loki labels: %w", err)
		}

		result, _ := json.Marshal(labels)
		return string(result), nil

	case "search_tempo_services":
		tempoDS, err := discoveryService.FindDatasourceByType("tempo")
		if err != nil {
			return "", fmt.Errorf("no Tempo datasource found: %w", err)
		}

		services, err := discoveryService.GetTempoServices(tempoDS.UID)
		if err != nil {
			return "", fmt.Errorf("failed to get Tempo services: %w", err)
		}

		result, _ := json.Marshal(services)
		return string(result), nil

	case "search_user":
		var params struct {
			LoginOrEmail string `json:"login_or_email"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid parameters: %w", err)
		}

		userInfo, err := discoveryService.SearchUser(params.LoginOrEmail)
		if err != nil {
			return "", fmt.Errorf("usuário não encontrado: %w", err)
		}

		result, _ := json.Marshal(userInfo)
		return string(result), nil

	case "search_dashboards":
		var params struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid parameters: %w", err)
		}

		dashboards, err := discoveryService.SearchDashboards(params.Query)
		if err != nil {
			return "", fmt.Errorf("failed to search dashboards: %w", err)
		}

		result, _ := json.Marshal(dashboards)
		return string(result), nil

	case "get_my_dashboards":
		var params struct {
			Filter string `json:"filter"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			params.Filter = "edit" // default to edit permission
		}
		if params.Filter == "" {
			params.Filter = "edit"
		}

		// Use logged user info to filter dashboards (including role for permission check)
		dashboards, err := discoveryService.GetUserDashboards(loggedUser.Login, loggedUser.Email, loggedUser.Role, params.Filter)
		if err != nil {
			return "", fmt.Errorf("failed to get user dashboards: %w", err)
		}

		result, _ := json.Marshal(dashboards)
		return string(result), nil

	case "create_dashboard":
		var dashData AdvancedDashData
		if err := json.Unmarshal(args, &dashData); err != nil {
			return "", fmt.Errorf("invalid dashboard parameters: %w", err)
		}

		// Resolve unique title (auto-handles collisions)
		dashData.Title = resolveUniqueTitle(discoveryService, dashData.Title)

		// Infer and enrich tags from dashboard content (backend safety net)
		dashData.Tags = inferTags(&dashData)

		// Set the requesting user for audit purposes (stored in dashboard metadata)
		impersonateUser := ""
		if loggedUser != nil && (loggedUser.Login != "" || loggedUser.Email != "") {
			dashData.RequestedBy = &DashboardRequester{
				Login: loggedUser.Login,
				Email: loggedUser.Email,
			}
			// Use login for impersonation (Grafana's X-Grafana-User header)
			if loggedUser.Login != "" {
				impersonateUser = loggedUser.Login
			} else {
				impersonateUser = loggedUser.Email
			}
		}

		// Create service with user impersonation for proper "Created by" attribution
		grafanaService := NewGrafanaServiceWithUser(settings.GrafanaURL, settings.GrafanaToken, impersonateUser)
		dashResult, err := grafanaService.CreateAdvancedDashboard(&dashData)
		if err != nil {
			result, _ := json.Marshal(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return string(result), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"success": true,
			"uid":     dashResult.UID,
			"id":      dashResult.ID,
			"title":   dashData.Title,
			"url":     dashResult.URL,
		})
		return string(result), nil

	default:
		return "", fmt.Errorf("unknown function: %s", funcName)
	}
}

// formatMetricsList formats a list of metrics for display
func formatMetricsList(metrics []string) string {
	if len(metrics) <= 20 {
		var sb strings.Builder
		for _, m := range metrics {
			sb.WriteString("- `")
			sb.WriteString(m)
			sb.WriteString("`\n")
		}
		return sb.String()
	}

	// Group by prefix for large lists
	groups := make(map[string][]string)
	for _, m := range metrics {
		parts := strings.SplitN(m, "_", 2)
		prefix := parts[0]
		groups[prefix] = append(groups[prefix], m)
	}

	var sb strings.Builder
	for prefix, group := range groups {
		sb.WriteString(fmt.Sprintf("**%s_*** (%d métricas)\n", prefix, len(group)))
		// Show first 5 of each group
		for i, m := range group {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("  ... e mais %d\n", len(group)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("  - `%s`\n", m))
		}
	}
	return sb.String()
}

// validateDashboardParams validates dashboard creation parameters
func validateDashboardParams(args json.RawMessage, discoveryService *DiscoveryService) string {
	var params struct {
		Title  string        `json:"title"`
		Folder string        `json:"folder"`
		Panels []PanelConfig `json:"panels"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "Parâmetros inválidos para criação de dashboard."
	}

	// Validate title
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return "O título do dashboard não pode ser vazio."
	}
	if len(title) < 3 {
		return "O título do dashboard deve ter pelo menos 3 caracteres."
	}
	if len(title) > 100 {
		return "O título do dashboard não pode ter mais de 100 caracteres."
	}

	// Check for invalid characters
	invalidChars := []string{"/", "\\", "<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(title, char) {
			return fmt.Sprintf("O título do dashboard contém caractere inválido: '%s'", char)
		}
	}

	// Validate folder name if provided
	folder := strings.TrimSpace(params.Folder)
	if folder != "" && strings.ToLower(folder) != "general" {
		if len(folder) < 2 {
			return "O nome da pasta deve ter pelo menos 2 caracteres."
		}
		if len(folder) > 50 {
			return "O nome da pasta não pode ter mais de 50 caracteres."
		}
		for _, char := range invalidChars {
			if strings.Contains(folder, char) {
				return fmt.Sprintf("O nome da pasta contém caractere inválido: '%s'", char)
			}
		}
	}

	// Validate number of panels (prevent huge dashboards)
	if len(params.Panels) > 50 {
		return "Dashboard não pode ter mais de 50 painéis para evitar sobrecarga."
	}

	// Validate panel queries for performance issues
	if len(params.Panels) > 0 {
		queryValidation := ValidatePanelQueries(params.Panels)
		if !queryValidation.Valid {
			// Return first error
			if len(queryValidation.Errors) > 0 {
				return fmt.Sprintf("Erro na query: %s", queryValidation.Errors[0])
			}
		}
		// Log warnings but don't block
		for _, warning := range queryValidation.Warnings {
			log.DefaultLogger.Warn("Query warning in dashboard creation", "warning", warning)
		}
	}

	return ""
}

// formatDatasourcesList formats datasources for display
func formatDatasourcesList(datasources []DatasourceInfo) string {
	var sb strings.Builder
	for _, ds := range datasources {
		defaultStr := ""
		if ds.IsDefault {
			defaultStr = " **(default)**"
		}
		sb.WriteString(fmt.Sprintf("- **%s** (tipo: `%s`, uid: `%s`)%s\n", ds.Name, ds.Type, ds.UID, defaultStr))
	}
	return sb.String()
}

// formatDashboardsList formats dashboards for display
func formatDashboardsList(dashboards []DashboardSearchResult) string {
	var sb strings.Builder
	for _, dash := range dashboards {
		folder := "General"
		if dash.Folder != "" {
			folder = dash.Folder
		}
		tags := ""
		if len(dash.Tags) > 0 {
			tags = fmt.Sprintf(" [tags: %s]", strings.Join(dash.Tags, ", "))
		}
		sb.WriteString(fmt.Sprintf("- **%s** (pasta: %s, uid: `%s`)%s\n", dash.Title, folder, dash.UID, tags))
	}
	return sb.String()
}

// formatUserDashboardsList formats dashboards with permission info for display
func formatUserDashboardsList(dashboards []DashboardWithPermission) string {
	var sb strings.Builder
	for _, dash := range dashboards {
		folder := "General"
		if dash.Folder != "" {
			folder = dash.Folder
		}

		// Permission badge
		permBadge := ""
		switch dash.Permission {
		case "owner":
			permBadge = " **[OWNER]**"
		case "admin":
			permBadge = " [Admin]"
		case "edit":
			permBadge = " [Pode editar]"
		default:
			permBadge = " [Visualização]"
		}

		// Created by info
		createdBy := ""
		if dash.CreatedBy != "" {
			createdBy = fmt.Sprintf(" (criado por: %s)", dash.CreatedBy)
		}

		sb.WriteString(fmt.Sprintf("- **%s**%s\n", dash.Title, permBadge))
		sb.WriteString(fmt.Sprintf("  - Pasta: %s | UID: `%s`%s\n", folder, dash.UID, createdBy))
	}
	return sb.String()
}

// getMetricPrefixes extracts unique prefixes from metrics
func getMetricPrefixes(metrics []string) []string {
	prefixSet := make(map[string]bool)
	for _, m := range metrics {
		parts := strings.SplitN(m, "_", 2)
		if len(parts) > 0 {
			prefixSet[parts[0]+"_"] = true
		}
	}

	prefixes := make([]string, 0, len(prefixSet))
	for p := range prefixSet {
		prefixes = append(prefixes, p)
	}
	return prefixes
}

// formatSampleMetrics returns a sample of metrics for display
func formatSampleMetrics(metrics []string, limit int) string {
	var sb strings.Builder
	for i, m := range metrics {
		if i >= limit {
			sb.WriteString(fmt.Sprintf("  ... e mais %d métricas\n", len(metrics)-limit))
			break
		}
		sb.WriteString(fmt.Sprintf("- `%s`\n", m))
	}
	return sb.String()
}

// enrichContextWithDefaults adds configured default datasources to the environment context
func enrichContextWithDefaults(ctx *EnvironmentContext, settings AppSettings) {
	if ctx == nil || len(ctx.Datasources) == 0 {
		return
	}

	// Separate datasources by type and identify defaults
	for _, ds := range ctx.Datasources {
		dsCopy := ds // Create copy to avoid pointer issues

		switch ds.Type {
		case "prometheus":
			if settings.DefaultPrometheusUID != "" && ds.UID == settings.DefaultPrometheusUID {
				ctx.DefaultPrometheus = &dsCopy
			} else {
				ctx.OtherPrometheus = append(ctx.OtherPrometheus, dsCopy)
			}
		case "loki":
			if settings.DefaultLokiUID != "" && ds.UID == settings.DefaultLokiUID {
				ctx.DefaultLoki = &dsCopy
			} else {
				ctx.OtherLoki = append(ctx.OtherLoki, dsCopy)
			}
		case "tempo":
			if settings.DefaultTempoUID != "" && ds.UID == settings.DefaultTempoUID {
				ctx.DefaultTempo = &dsCopy
			} else {
				ctx.OtherTempo = append(ctx.OtherTempo, dsCopy)
			}
		}
	}

	// If no default was configured but there are datasources of that type, use the first one or the Grafana default
	if ctx.DefaultPrometheus == nil && len(ctx.OtherPrometheus) > 0 {
		// Check if any is marked as Grafana default
		for i, ds := range ctx.OtherPrometheus {
			if ds.IsDefault {
				ctx.DefaultPrometheus = &ctx.OtherPrometheus[i]
				ctx.OtherPrometheus = append(ctx.OtherPrometheus[:i], ctx.OtherPrometheus[i+1:]...)
				break
			}
		}
		// If still no default, use the first one
		if ctx.DefaultPrometheus == nil {
			ctx.DefaultPrometheus = &ctx.OtherPrometheus[0]
			ctx.OtherPrometheus = ctx.OtherPrometheus[1:]
		}
	}

	if ctx.DefaultLoki == nil && len(ctx.OtherLoki) > 0 {
		for i, ds := range ctx.OtherLoki {
			if ds.IsDefault {
				ctx.DefaultLoki = &ctx.OtherLoki[i]
				ctx.OtherLoki = append(ctx.OtherLoki[:i], ctx.OtherLoki[i+1:]...)
				break
			}
		}
		if ctx.DefaultLoki == nil {
			ctx.DefaultLoki = &ctx.OtherLoki[0]
			ctx.OtherLoki = ctx.OtherLoki[1:]
		}
	}

	if ctx.DefaultTempo == nil && len(ctx.OtherTempo) > 0 {
		for i, ds := range ctx.OtherTempo {
			if ds.IsDefault {
				ctx.DefaultTempo = &ctx.OtherTempo[i]
				ctx.OtherTempo = append(ctx.OtherTempo[:i], ctx.OtherTempo[i+1:]...)
				break
			}
		}
		if ctx.DefaultTempo == nil {
			ctx.DefaultTempo = &ctx.OtherTempo[0]
			ctx.OtherTempo = ctx.OtherTempo[1:]
		}
	}

	log.DefaultLogger.Info("Context enriched with defaults",
		"defaultPrometheus", ctx.DefaultPrometheus != nil,
		"otherPrometheus", len(ctx.OtherPrometheus),
		"defaultLoki", ctx.DefaultLoki != nil,
		"otherLoki", len(ctx.OtherLoki),
		"defaultTempo", ctx.DefaultTempo != nil,
		"otherTempo", len(ctx.OtherTempo))
}

// resolveUniqueTitle checks if a dashboard title already exists and resolves collisions
// by appending a numeric suffix. Returns the resolved (unique) title.
func resolveUniqueTitle(ds *DiscoveryService, title string) string {
	if ds == nil {
		return title
	}

	// Try the original title first
	exists, _ := ds.DashboardExists(title)
	if !exists {
		return title
	}

	log.DefaultLogger.Info("Dashboard title collision detected, resolving", "title", title)

	// Try suffixes (2) through (10)
	for i := 2; i <= 10; i++ {
		candidate := fmt.Sprintf("%s (%d)", title, i)
		exists, _ := ds.DashboardExists(candidate)
		if !exists {
			log.DefaultLogger.Info("Resolved dashboard title", "original", title, "resolved", candidate)
			return candidate
		}
	}

	// Fallback: use timestamp
	candidate := fmt.Sprintf("%s (%d)", title, time.Now().Unix())
	log.DefaultLogger.Info("Resolved dashboard title with timestamp", "original", title, "resolved", candidate)
	return candidate
}

// inferTags analyzes the dashboard title, panel queries, and panel types to generate
// relevant tags. It merges inferred tags with any tags already provided by the AI,
// guaranteeing that every dashboard has meaningful, content-based tags.
func inferTags(dashData *AdvancedDashData) []string {
	existing := make(map[string]bool)
	for _, t := range dashData.Tags {
		existing[strings.ToLower(t)] = true
	}

	// helper: add tag only if not already present
	add := func(tag string) {
		tag = strings.ToLower(tag)
		if !existing[tag] {
			existing[tag] = true
			dashData.Tags = append(dashData.Tags, tag)
		}
	}

	// Combine title + all queries into a single searchable string
	blob := strings.ToLower(dashData.Title)
	for _, p := range dashData.Panels {
		blob += " " + strings.ToLower(p.Query) + " " + strings.ToLower(p.Title)
	}

	// --- Keyword → tag mapping (order doesn't matter, all are checked) ---
	keywordTags := []struct {
		keywords []string
		tag      string
	}{
		// Infrastructure resources
		{[]string{"cpu", "processor", "system_cpu"}, "cpu"},
		{[]string{"memory", "memória", "mem_", "system_memory"}, "memory"},
		{[]string{"disk", "disco", "filesystem", "system_disk", "fs_"}, "disk"},
		{[]string{"network", "rede", "net_", "system_network", "tcp_", "udp_"}, "network"},
		{[]string{"node_", "host", "machine"}, "infrastructure"},
		{[]string{"container_", "docker", "k8s", "kube", "pod"}, "containers"},

		// Protocols / services
		{[]string{"http_", "http_request", "http_response"}, "http"},
		{[]string{"grpc_", "grpc."}, "grpc"},
		{[]string{"dns_"}, "dns"},

		// Methodologies
		{[]string{"golden signal", "latency", "traffic", "errors", "saturation"}, "golden-signals"},
		{[]string{"red method", "rate", "duration"}, "red-method"},
		{[]string{"use method", "utilization"}, "use-method"},

		// Observability pillars
		{[]string{"log", "loki", "logql"}, "logs"},
		{[]string{"trace", "tempo", "traceql", "span"}, "traces"},

		// Application
		{[]string{"jvm", "java", "heap"}, "jvm"},
		{[]string{"process_", "runtime"}, "process"},
		{[]string{"go_", "golang"}, "golang"},
		{[]string{"nginx"}, "nginx"},
		{[]string{"postgres", "pg_", "mysql", "database", "db_"}, "database"},
	}

	for _, kt := range keywordTags {
		for _, kw := range kt.keywords {
			if strings.Contains(blob, kw) {
				add(kt.tag)
				break
			}
		}
	}

	// --- Panel-type heuristics for methodology detection ---
	panelTypeCounts := make(map[string]int)
	for _, p := range dashData.Panels {
		panelTypeCounts[p.Type]++
	}

	hasGauge := panelTypeCounts["gauge"] > 0
	hasStat := panelTypeCounts["stat"] > 0
	hasTimeseries := panelTypeCounts["timeseries"] > 0
	hasHeatmap := panelTypeCounts["heatmap"] > 0

	// Golden Signals pattern: stat + timeseries + (heatmap or gauge), 4+ panels
	if len(dashData.Panels) >= 4 && hasStat && hasTimeseries && (hasHeatmap || hasGauge) {
		add("golden-signals")
	}
	// USE pattern: gauge + timeseries together
	if hasGauge && hasTimeseries {
		add("use-method")
	}
	// RED pattern: stat + timeseries (rates and errors)
	if hasStat && hasTimeseries && !hasGauge {
		add("red-method")
	}

	// If after all inference we still have no tags (beyond what AI sent), add a generic one from the title
	// This shouldn't normally happen but acts as a final safety net
	if len(dashData.Tags) == 0 {
		add("monitoring")
	}

	log.DefaultLogger.Info("Tags inferred for dashboard",
		"title", dashData.Title,
		"finalTags", dashData.Tags,
		"totalCount", len(dashData.Tags))

	return dashData.Tags
}
