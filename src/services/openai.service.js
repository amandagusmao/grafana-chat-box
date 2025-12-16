import OpenAI from 'openai';

export class OpenAIService {
  constructor(apiKey) {
    this.client = new OpenAI({
      apiKey: apiKey,
    });
    
    this.systemPrompt = `Você é um especialista sênior em Grafana e Observabilidade, com amplo domínio em SRE e DevOps. Seu conhecimento abrange:

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

Foque sempre em entregar valor real, com sugestões técnicas precisas e dashboards bem estruturados.`;
  }

  async processChat(messages, dashboardContext = {}) {
    try {
      const allMessages = [
        { role: 'system', content: this.systemPrompt },
        ...messages
      ];

      const completion = await this.client.chat.completions.create({
        model: 'gpt-4o-mini',
        messages: allMessages,
        tools: [{
          type: 'function',
          function: {
            name: 'create_dashboard',
            description: 'Criar dashboard no Grafana quando todas as informações necessárias foram coletadas',
            parameters: {
              type: 'object',
              properties: {
                title: {
                  type: 'string',
                  description: 'Título do dashboard'
                },
                datasource: {
                  type: 'string',
                  description: 'Nome da fonte de dados (ex: prometheus, influxdb)'
                },
                metrics: {
                  type: 'array',
                  items: { type: 'string' },
                  description: 'Lista de métricas a serem visualizadas'
                },
                folder: {
                  type: 'string',
                  description: 'Nome da pasta onde criar o dashboard (opcional)'
                }
              },
              required: ['title', 'datasource', 'metrics']
            }
          }
        }],
        tool_choice: 'auto',
        temperature: 0.7,
        max_completion_tokens: 500
      });

      const message = completion.choices[0].message;
      
      // Check if AI wants to call the create_dashboard function
      if (message.tool_calls && message.tool_calls.length > 0) {
        const toolCall = message.tool_calls[0];
        if (toolCall.function.name === 'create_dashboard') {
          const dashboardData = JSON.parse(toolCall.function.arguments);
          
          return {
            type: 'function_call',
            function: 'create_dashboard',
            data: dashboardData,
            response: `Perfeito! Tenho todas as informações necessárias. Vou criar o dashboard "${dashboardData.title}" agora...`
          };
        }
      }

      // Regular chat response
      return {
        type: 'message',
        response: message.content
      };

    } catch (error) {
      console.error('OpenAI API error:', error);
      console.error('Full error details:', JSON.stringify(error, null, 2));
      throw new Error(`Erro na comunicação com IA: ${error.message}`);
    }
  }
}