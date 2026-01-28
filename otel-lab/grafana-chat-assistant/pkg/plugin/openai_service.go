package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

const baseSystemPrompt = `Você é um especialista sênior em Grafana e Observabilidade, com amplo domínio em SRE e DevOps. Seu conhecimento abrange:

**EXPERTISE TÉCNICA:**
- Grafana (Dashboards, Panels, Variables, Transformations, Alerts, Annotations)
- Prometheus/PromQL, Loki/LogQL, Tempo/Traces
- Elasticsearch, InfluxDB, OpenTelemetry
- Integrações com Dynatrace, Splunk e outras ferramentas
- RED Method, USE Method, Golden Signals

**SEU PAPEL:**
Auxiliar usuários de todos os níveis (iniciantes a especialistas) na criação de dashboards eficazes, sempre priorizando boas práticas e valor operacional.

**COMPORTAMENTO ESPERADO:**

1. **Linguagem Natural & Assertiva:**
   - Tom profissional, técnico e colaborativo
   - Linguagem clara, sem jargões desnecessários
   - Seja direto e objetivo, evite respostas genéricas

2. **Descoberta de Métricas:**
   - Use a ferramenta 'search_metrics' para buscar métricas disponíveis antes de criar dashboards
   - SEMPRE verifique quais métricas existem no ambiente antes de propor um dashboard
   - Nunca invente métricas - use apenas as que existem no ambiente

3. **Sugestão de Visualizações:**
   - Indique tipo de painel adequado (timeseries, stat, gauge, table, heatmap, logs, traces)
   - Justifique a escolha (tendência, comparação, valor atual, distribuição)
   - Sugira thresholds, cores e alertas quando apropriado

4. **Criação de Dashboards:**
   - Para pedidos vagos, use search_metrics para descobrir o que está disponível
   - Proponha estrutura completa com organização lógica dos painéis
   - Agrupe por contexto (overview, performance, erros, infraestrutura)
   - Use as métricas REAIS descobertas pelo search_metrics

**PROCESSO DE CRIAÇÃO:**
1. Use list_datasources para ver fontes de dados disponíveis
2. Use search_metrics com padrões relevantes (ex: "node_", "http_", "process_")
3. Analise as métricas encontradas
4. Proponha um dashboard usando create_dashboard com métricas REAIS

**BOAS PRÁTICAS OBRIGATÓRIAS:**
- Evite dashboards poluídos
- Priorize métricas acionáveis
- Incentive uso de variables e templates
- Mantenha consistência visual
- NUNCA use métricas que não foram confirmadas pelo search_metrics`

// Request/Response types for API

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []Tool        `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float32       `json:"temperature,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ChatCompletionChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type ChatCompletionInnerResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
}

type ChatCompletionResponse struct {
	Response ChatCompletionInnerResponse `json:"response"`
}

// AIService handles AI API interactions
type AIService struct {
	httpClient *http.Client
	endpoint   string
	model      string
}

// AIResponse represents the response from AI processing
type AIResponse struct {
	Type         string            `json:"type"`
	Function     string            `json:"function,omitempty"`
	FunctionArgs json.RawMessage   `json:"functionArgs,omitempty"`
	Data         DashboardData     `json:"data,omitempty"`
	AdvancedData *AdvancedDashData `json:"advancedData,omitempty"`
	Response     string            `json:"response"`
}

// DashboardData represents the basic data needed to create a dashboard (legacy)
type DashboardData struct {
	Title      string   `json:"title"`
	Datasource string   `json:"datasource"`
	Metrics    []string `json:"metrics"`
	Folder     string   `json:"folder,omitempty"`
}

// AdvancedDashData represents advanced dashboard configuration
type AdvancedDashData struct {
	Title         string           `json:"title"`
	Description   string           `json:"description,omitempty"`
	DatasourceUID string           `json:"datasource_uid"`
	Folder        string           `json:"folder,omitempty"`
	Panels        []PanelConfig    `json:"panels"`
	Variables     []VariableConfig `json:"variables,omitempty"`
	TimeRange     *TimeRangeConfig `json:"time_range,omitempty"`
	Refresh       string           `json:"refresh,omitempty"`
	Tags          []string         `json:"tags,omitempty"`
	// RequestedBy contains info about the user who requested the dashboard creation (set by backend, not AI)
	RequestedBy *DashboardRequester `json:"-"`
}

// DashboardRequester contains information about who requested the dashboard creation
type DashboardRequester struct {
	Login string `json:"login"`
	Email string `json:"email"`
}

// PanelConfig represents a panel configuration
type PanelConfig struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type"` // timeseries, stat, gauge, table, heatmap, logs, traces, bargauge, piechart
	Query       string            `json:"query"`
	QueryType   string            `json:"query_type,omitempty"` // promql, logql, traceql
	Unit        string            `json:"unit,omitempty"`
	Thresholds  []ThresholdConfig `json:"thresholds,omitempty"`
	Width       int               `json:"width,omitempty"`  // Grid width (1-24, default 12)
	Height      int               `json:"height,omitempty"` // Grid height (default 8)
}

// ThresholdConfig represents threshold configuration
type ThresholdConfig struct {
	Value float64 `json:"value"`
	Color string  `json:"color"` // green, yellow, orange, red, or hex color
}

// VariableConfig represents a dashboard variable
type VariableConfig struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	Type        string `json:"type"` // query, custom, interval, datasource
	Query       string `json:"query,omitempty"`
	Options     string `json:"options,omitempty"`   // For custom type: comma-separated values
	Datasource  string `json:"datasource,omitempty"`
	Refresh     int    `json:"refresh,omitempty"`   // 0=never, 1=on dashboard load, 2=on time range change
	Multi       bool   `json:"multi,omitempty"`
	IncludeAll  bool   `json:"include_all,omitempty"`
}

// TimeRangeConfig represents time range configuration
type TimeRangeConfig struct {
	From string `json:"from"` // e.g., "now-1h", "now-24h"
	To   string `json:"to"`   // e.g., "now"
}

// NewAIService creates a new AI service instance
func NewAIService(endpoint, model string) *AIService {
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &AIService{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		endpoint: endpoint,
		model:    model,
	}
}

// buildSystemPrompt builds the system prompt with environment context
func buildSystemPrompt(ctx *EnvironmentContext) string {
	var sb strings.Builder
	sb.WriteString(baseSystemPrompt)
	sb.WriteString("\n\n")

	if ctx != nil {
		sb.WriteString("## CONTEXTO DO AMBIENTE ATUAL\n\n")

		// Show default datasources prominently
		sb.WriteString("### Datasources Padrão (usar se o usuário não especificar):\n")

		if ctx.DefaultPrometheus != nil {
			sb.WriteString(fmt.Sprintf("- **Prometheus**: %s (uid: `%s`) - PADRÃO\n", ctx.DefaultPrometheus.Name, ctx.DefaultPrometheus.UID))
		}
		if ctx.DefaultLoki != nil {
			sb.WriteString(fmt.Sprintf("- **Loki**: %s (uid: `%s`) - PADRÃO\n", ctx.DefaultLoki.Name, ctx.DefaultLoki.UID))
		}
		if ctx.DefaultTempo != nil {
			sb.WriteString(fmt.Sprintf("- **Tempo**: %s (uid: `%s`) - PADRÃO\n", ctx.DefaultTempo.Name, ctx.DefaultTempo.UID))
		}
		sb.WriteString("\n")

		// Show other available datasources of each type
		hasOthers := len(ctx.OtherPrometheus) > 0 || len(ctx.OtherLoki) > 0 || len(ctx.OtherTempo) > 0
		if hasOthers {
			sb.WriteString("### Outros Datasources Disponíveis:\n")
			sb.WriteString("*O usuário pode solicitar explicitamente um destes datasources ao criar dashboards.*\n\n")

			if len(ctx.OtherPrometheus) > 0 {
				sb.WriteString("**Prometheus alternativos:**\n")
				for _, ds := range ctx.OtherPrometheus {
					sb.WriteString(fmt.Sprintf("  - %s (uid: `%s`)\n", ds.Name, ds.UID))
				}
			}
			if len(ctx.OtherLoki) > 0 {
				sb.WriteString("**Loki alternativos:**\n")
				for _, ds := range ctx.OtherLoki {
					sb.WriteString(fmt.Sprintf("  - %s (uid: `%s`)\n", ds.Name, ds.UID))
				}
			}
			if len(ctx.OtherTempo) > 0 {
				sb.WriteString("**Tempo alternativos:**\n")
				for _, ds := range ctx.OtherTempo {
					sb.WriteString(fmt.Sprintf("  - %s (uid: `%s`)\n", ds.Name, ds.UID))
				}
			}
			sb.WriteString("\n")
		}

		// Show other datasource types (non Prometheus/Loki/Tempo)
		var otherTypes []DatasourceInfo
		for _, ds := range ctx.Datasources {
			if ds.Type != "prometheus" && ds.Type != "loki" && ds.Type != "tempo" {
				otherTypes = append(otherTypes, ds)
			}
		}
		if len(otherTypes) > 0 {
			sb.WriteString("### Outros Tipos de Datasources:\n")
			for _, ds := range otherTypes {
				sb.WriteString(fmt.Sprintf("- %s (tipo: %s, uid: `%s`)\n", ds.Name, ds.Type, ds.UID))
			}
			sb.WriteString("\n")
		}

		if len(ctx.LokiLabels) > 0 {
			sb.WriteString("### Labels Loki Disponíveis:\n")
			sb.WriteString(strings.Join(ctx.LokiLabels, ", "))
			sb.WriteString("\n\n")
		}

		if len(ctx.TempoServices) > 0 {
			sb.WriteString("### Serviços Tempo Disponíveis:\n")
			sb.WriteString(strings.Join(ctx.TempoServices, ", "))
			sb.WriteString("\n\n")
		}

		if ctx.UserInfo != nil {
			sb.WriteString("### Informações do Usuário:\n")
			sb.WriteString(fmt.Sprintf("- Login: %s\n", ctx.UserInfo.Login))
			sb.WriteString(fmt.Sprintf("- Role: %s\n", ctx.UserInfo.Role))
			if ctx.UserInfo.IsGrafanaAdmin {
				sb.WriteString("- Admin: Sim\n")
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString(`## REGRAS IMPORTANTES:
1. Use 'search_metrics' para descobrir métricas ANTES de criar dashboards
2. Use 'list_datasources' para ver os datasources disponíveis
3. Para 4 Golden Signals: latency, traffic, errors, saturation
4. NUNCA invente métricas - use apenas as descobertas pelo search_metrics
5. IMPORTANTE: Comece buscando com padrões genéricos como: system_, scrape_, up
6. Se não encontrar métricas, tente padrões mais amplos ou busque "" para ver todas
7. Para infraestrutura OpenTelemetry, prefixos comuns: system_cpu_, system_memory_, system_disk_, system_network_

## REGRAS DE DATASOURCES:
1. **Se o usuário NÃO especificar qual datasource usar**: Use o datasource PADRÃO listado acima
2. **Se o usuário especificar um datasource por nome**: Use o UID correspondente da lista
3. **Sempre informe ao usuário** qual datasource será usado ao criar um dashboard
4. **Se houver múltiplos datasources** do mesmo tipo, pergunte ao usuário qual prefere (ou use o padrão)
5. Ao criar dashboards, use o UID do datasource, não o nome

## BOAS PRÁTICAS DE QUERIES (OBRIGATÓRIO):
1. **Counters (_total, _count, _bucket)**: SEMPRE use rate() ou increase() - nunca use valores brutos
   - Correto: rate(http_requests_total[5m])
   - Errado: http_requests_total
2. **Time ranges**: Use ranges razoáveis (1m a 1h para rate). Evite ranges muito longos sem agregação
3. **Agregação**: Use by() ou without() para reduzir cardinalidade em queries complexas
4. **Regex**: Evite regex muito amplos como {job=~".*"} - seja específico
5. **LogQL**: SEMPRE inclua filtros de label no stream selector - nunca use {} vazio
6. **Limite de painéis**: Máximo de 50 painéis por dashboard
7. **Histogramas**: Use histogram_quantile() corretamente com rate() nos buckets
`)
	sb.WriteString("\n")
	sb.WriteString(`## EXEMPLOS DE QUERIES OTIMIZADAS:
- CPU: rate(process_cpu_seconds_total[5m]) * 100
- Memória: process_resident_memory_bytes / 1024 / 1024
- Taxa de erros: sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
- Latência p99: histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))
`)

	return sb.String()
}

// getTools returns the available tools for the AI
func getTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "search_metrics",
				Description: "Buscar métricas Prometheus disponíveis por padrão. Use SEMPRE antes de criar dashboards para descobrir quais métricas existem. Exemplos de padrões: 'node_' para métricas de host, 'http_' para métricas HTTP, 'process_' para métricas de processo.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Padrão de busca para filtrar métricas (ex: node_, http_, process_, container_)",
						},
					},
					"required": []string{"pattern"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_datasources",
				Description: "Listar todos os datasources configurados no Grafana com seus tipos e UIDs",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "search_loki_labels",
				Description: "Listar labels disponíveis no Loki para filtrar logs",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "search_tempo_services",
				Description: "Listar serviços com traces disponíveis no Tempo",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_user_permissions",
				Description: "Obter informações e permissões do usuário do Service Account configurado no plugin",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "search_user",
				Description: "Buscar informações de um usuário específico do Grafana por email ou login. Retorna nome, role, organizações e se é admin.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"login_or_email": map[string]interface{}{
							"type":        "string",
							"description": "Email ou login do usuário a ser buscado",
						},
					},
					"required": []string{"login_or_email"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "search_dashboards",
				Description: "Buscar dashboards existentes no Grafana. Use para verificar se já existe um dashboard sobre determinado assunto antes de criar um novo, ou para listar dashboards disponíveis.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Termo de busca para filtrar dashboards pelo título (opcional, deixe vazio para listar todos)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_my_dashboards",
				Description: "Listar dashboards onde o usuário logado é dono (criador) ou tem permissão de edição. Use quando o usuário perguntar sobre 'meus dashboards', 'dashboards que eu criei', 'dashboards que posso editar', etc.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filter": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"owner", "edit", "all"},
							"description": "Filtro de permissão: 'owner' = apenas dashboards criados pelo usuário, 'edit' = dashboards que o usuário pode editar (inclui owner), 'all' = todos os dashboards visíveis",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "create_dashboard",
				Description: "Criar dashboard no Grafana com configurações avançadas. Use APENAS após descobrir as métricas disponíveis com search_metrics. IMPORTANTE: Antes de criar, use search_dashboards para verificar se já existe um dashboard similar.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Título do dashboard",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Descrição do dashboard (opcional)",
						},
						"datasource_uid": map[string]interface{}{
							"type":        "string",
							"description": "UID do datasource principal (obter via list_datasources)",
						},
						"folder": map[string]interface{}{
							"type":        "string",
							"description": "Nome da pasta onde criar o dashboard (opcional, default: General)",
						},
						"panels": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"title": map[string]interface{}{
										"type":        "string",
										"description": "Título do painel",
									},
									"description": map[string]interface{}{
										"type":        "string",
										"description": "Descrição do painel (opcional)",
									},
									"type": map[string]interface{}{
										"type":        "string",
										"enum":        []string{"timeseries", "stat", "gauge", "table", "heatmap", "logs", "bargauge", "piechart"},
										"description": "Tipo de visualização",
									},
									"query": map[string]interface{}{
										"type":        "string",
										"description": "Query PromQL, LogQL ou TraceQL",
									},
									"unit": map[string]interface{}{
										"type":        "string",
										"description": "Unidade de medida (bytes, percent, seconds, short, etc.)",
									},
									"thresholds": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"value": map[string]interface{}{
													"type":        "number",
													"description": "Valor do threshold",
												},
												"color": map[string]interface{}{
													"type":        "string",
													"description": "Cor (green, yellow, orange, red)",
												},
											},
										},
										"description": "Thresholds para coloração",
									},
									"width": map[string]interface{}{
										"type":        "integer",
										"description": "Largura do painel (1-24, default: 12)",
									},
									"height": map[string]interface{}{
										"type":        "integer",
										"description": "Altura do painel (default: 8)",
									},
								},
								"required": []string{"title", "type", "query"},
							},
							"description": "Lista de painéis do dashboard",
						},
						"variables": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"type":        "string",
										"description": "Nome da variável (sem $)",
									},
									"label": map[string]interface{}{
										"type":        "string",
										"description": "Label de exibição",
									},
									"type": map[string]interface{}{
										"type":        "string",
										"enum":        []string{"query", "custom", "interval", "datasource"},
										"description": "Tipo da variável",
									},
									"query": map[string]interface{}{
										"type":        "string",
										"description": "Query para popular valores (para tipo query)",
									},
									"options": map[string]interface{}{
										"type":        "string",
										"description": "Valores separados por vírgula (para tipo custom)",
									},
									"multi": map[string]interface{}{
										"type":        "boolean",
										"description": "Permitir múltipla seleção",
									},
									"include_all": map[string]interface{}{
										"type":        "boolean",
										"description": "Incluir opção 'All'",
									},
								},
								"required": []string{"name", "type"},
							},
							"description": "Variáveis de template do dashboard",
						},
						"refresh": map[string]interface{}{
							"type":        "string",
							"description": "Intervalo de refresh (5s, 10s, 30s, 1m, 5m, etc.)",
						},
						"tags": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "Tags do dashboard",
						},
					},
					"required": []string{"title", "datasource_uid", "panels"},
				},
			},
		},
	}
}

// ProcessChat processes the chat messages with the AI API (legacy - without context)
func (s *AIService) ProcessChat(messages []Message, token string) (*AIResponse, error) {
	return s.ProcessChatWithContext(messages, token, nil)
}

// ProcessChatWithContext processes chat with environment context
func (s *AIService) ProcessChatWithContext(messages []Message, token string, ctx *EnvironmentContext) (*AIResponse, error) {
	systemPrompt := buildSystemPrompt(ctx)

	// Convert messages to API format
	chatMessages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	for _, msg := range messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "assistant"
		}
		chatMessages = append(chatMessages, ChatMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	return s.callAPI(chatMessages, token)
}

// ProcessChatWithMessages processes chat with pre-built messages (for tool call continuation)
func (s *AIService) ProcessChatWithMessages(chatMessages []ChatMessage, token string) (*AIResponse, error) {
	return s.callAPI(chatMessages, token)
}

// callAPI makes the actual API call
func (s *AIService) callAPI(chatMessages []ChatMessage, token string) (*AIResponse, error) {
	tools := getTools()

	// Build request
	reqBody := ChatCompletionRequest{
		Model:       s.model,
		Messages:    chatMessages,
		Tools:       tools,
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.DefaultLogger.Error("Failed to marshal request", "error", err)
		return nil, err
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/iagen-textgenerator/v1/text/generate", s.endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.DefaultLogger.Error("Failed to create request", "error", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.DefaultLogger.Error("AI API request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.DefaultLogger.Error("Failed to read response body", "error", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.DefaultLogger.Error("AI API returned error", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("AI API error: %s", string(body))
	}

	// Parse response
	var apiResp ChatCompletionResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.DefaultLogger.Error("Failed to parse response", "error", err, "body", string(body))
		return nil, err
	}

	if len(apiResp.Response.Choices) == 0 {
		return &AIResponse{
			Type:     "message",
			Response: "Desculpe, não consegui processar sua mensagem. Tente novamente.",
		}, nil
	}

	choice := apiResp.Response.Choices[0]

	// Check if AI wants to call a function
	if len(choice.Message.ToolCalls) > 0 {
		toolCall := choice.Message.ToolCalls[0]
		funcName := toolCall.Function.Name

		log.DefaultLogger.Info("AI requested tool call", "function", funcName)

		return &AIResponse{
			Type:         "function_call",
			Function:     funcName,
			FunctionArgs: json.RawMessage(toolCall.Function.Arguments),
			Response:     choice.Message.Content,
		}, nil
	}

	// Regular message response
	return &AIResponse{
		Type:     "message",
		Response: choice.Message.Content,
	}, nil
}

// ContinueWithToolResult continues the conversation after a tool call
func (s *AIService) ContinueWithToolResult(
	originalMessages []ChatMessage,
	toolCallID string,
	toolName string,
	toolResult string,
	token string,
) (*AIResponse, error) {
	// Add the tool result as a message
	messages := append(originalMessages, ChatMessage{
		Role:       "tool",
		Content:    toolResult,
		ToolCallID: toolCallID,
	})

	return s.callAPI(messages, token)
}
