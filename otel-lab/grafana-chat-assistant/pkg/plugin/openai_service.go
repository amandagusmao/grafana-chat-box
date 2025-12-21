package plugin

import (
	"context"
	"encoding/json"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	openai "github.com/sashabaranov/go-openai"
)

const systemPrompt = `Você é um especialista sênior em Grafana e Observabilidade, com amplo domínio em SRE e DevOps. Seu conhecimento abrange:

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

2. **Recomendação de Métricas:**
   - Sempre considere tipo de sistema (API, frontend, banco, infra, K8s, cloud)
   - Foque no objetivo (performance, disponibilidade, erro, custo, capacidade)
   - Sugira métricas acionáveis baseadas em boas práticas

3. **Sugestão de Visualizações:**
   - Indique tipo de painel adequado (time series, stat, gauge, bar chart, heatmap, table)
   - Justifique a escolha (tendência, comparação, valor atual, distribuição)
   - Sugira thresholds, cores e alertas quando apropriado

4. **Criação de Dashboards:**
   - Para pedidos vagos (ex: "dashboard para minha API"), assuma boas práticas padrão
   - Faça no máximo 2-3 perguntas essenciais, se necessário
   - Proponha estrutura completa com organização lógica dos painéis
   - Agrupe por contexto (overview, performance, erros, infraestrutura)
   - Sempre sugira melhorias além do pedido inicial

**PROCESSO DE CRIAÇÃO:**
Quando o usuário quiser criar um dashboard:
1. Entenda o contexto/sistema alvo
2. Se necessário, faça perguntas específicas sobre:
   - Tipo de aplicação/sistema
   - Fonte de dados disponível
   - Foco principal (performance, SLO, troubleshooting)
3. Quando tiver informações suficientes, chame create_dashboard

**BOAS PRÁTICAS OBRIGATÓRIAS:**
- Evite dashboards poluídos
- Priorize métricas acionáveis
- Incentive uso de variables e templates
- Mantenha consistência visual
- Nunca invente métricas inexistentes - seja explícito sobre limitações

**EXEMPLO DE INTERAÇÃO:**
Usuário: "Preciso de um dashboard para minha API"
Você: "Perfeito! Vou criar um dashboard robusto para sua API. Para otimizar as métricas, qual fonte de dados você está usando (Prometheus, InfluxDB, etc.)? E que tipo de API é (REST, GraphQL, microserviço)?"

Foque sempre em entregar valor real, com sugestões técnicas precisas e dashboards bem estruturados.`

// OpenAIService handles OpenAI API interactions
type OpenAIService struct {
	client *openai.Client
	model  string
}

// AIResponse represents the response from OpenAI processing
type AIResponse struct {
	Type     string        `json:"type"`
	Function string        `json:"function,omitempty"`
	Data     DashboardData `json:"data,omitempty"`
	Response string        `json:"response"`
}

// DashboardData represents the data needed to create a dashboard
type DashboardData struct {
	Title      string   `json:"title"`
	Datasource string   `json:"datasource"`
	Metrics    []string `json:"metrics"`
	Folder     string   `json:"folder,omitempty"`
}

// NewOpenAIService creates a new OpenAI service instance
func NewOpenAIService(apiKey, endpoint, model string) *OpenAIService {
	var client *openai.Client

	if endpoint != "" {
		// Use custom endpoint (compatible with OpenAI API format)
		config := openai.DefaultConfig(apiKey)
		config.BaseURL = endpoint
		client = openai.NewClientWithConfig(config)
	} else {
		// Use default OpenAI endpoint
		client = openai.NewClient(apiKey)
	}

	// Default model if not specified
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &OpenAIService{
		client: client,
		model:  model,
	}
}

// ProcessChat processes the chat messages with OpenAI
func (s *OpenAIService) ProcessChat(messages []Message) (*AIResponse, error) {
	// Convert messages to OpenAI format
	chatMessages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	for _, msg := range messages {
		role := openai.ChatMessageRoleUser
		if msg.Role == "assistant" {
			role = openai.ChatMessageRoleAssistant
		}
		chatMessages = append(chatMessages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// Define the function for creating dashboards
	functions := []openai.FunctionDefinition{
		{
			Name:        "create_dashboard",
			Description: "Criar dashboard no Grafana quando todas as informações necessárias foram coletadas",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Título do dashboard",
					},
					"datasource": map[string]interface{}{
						"type":        "string",
						"description": "Nome da fonte de dados (ex: prometheus, influxdb)",
					},
					"metrics": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Lista de métricas a serem visualizadas",
					},
					"folder": map[string]interface{}{
						"type":        "string",
						"description": "Nome da pasta onde criar o dashboard (opcional)",
					},
				},
				"required": []string{"title", "datasource", "metrics"},
			},
		},
	}

	// Create tools from functions
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionDefinition{
				Name:        functions[0].Name,
				Description: functions[0].Description,
				Parameters:  functions[0].Parameters,
			},
		},
	}

	// Call OpenAI API
	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.model,
			Messages:    chatMessages,
			Tools:       tools,
			MaxTokens:   500,
			Temperature: 0.7,
		},
	)
	if err != nil {
		log.DefaultLogger.Error("OpenAI API error", "error", err)
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return &AIResponse{
			Type:     "message",
			Response: "Desculpe, não consegui processar sua mensagem. Tente novamente.",
		}, nil
	}

	choice := resp.Choices[0]

	// Check if AI wants to call a function
	if len(choice.Message.ToolCalls) > 0 {
		toolCall := choice.Message.ToolCalls[0]
		if toolCall.Function.Name == "create_dashboard" {
			var dashboardData DashboardData
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &dashboardData); err != nil {
				log.DefaultLogger.Error("Failed to parse function arguments", "error", err)
				return nil, err
			}

			return &AIResponse{
				Type:     "function_call",
				Function: "create_dashboard",
				Data:     dashboardData,
				Response: "Perfeito! Tenho todas as informações necessárias. Vou criar o dashboard \"" + dashboardData.Title + "\" agora...",
			}, nil
		}
	}

	// Regular message response
	return &AIResponse{
		Type:     "message",
		Response: choice.Message.Content,
	}, nil
}
