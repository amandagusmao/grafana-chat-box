import OpenAI from 'openai';

export class OpenAIService {
  constructor(apiKey) {
    this.client = new OpenAI({
      apiKey: apiKey,
    });
    
    this.systemPrompt = `Você é um assistente especializado em ajudar usuários a criar dashboards no Grafana.

Sua missão é conversar com o usuário de forma natural e coletar as seguintes informações essenciais:
1. **Título do dashboard** - Nome que o usuário quer dar ao dashboard
2. **Fonte de dados (datasource)** - Qual sistema de monitoramento usar (ex: Prometheus, InfluxDB, etc.)
3. **Métricas** - Quais métricas específicas o usuário quer visualizar
4. **Pasta (folder)** - Onde organizar o dashboard no Grafana

IMPORTANTE:
- Seja conversacional e amigável
- Faça uma pergunta por vez para não sobrecarregar o usuário
- Se o usuário não souber algo técnico, ajude com sugestões
- Quando tiver TODAS as informações essenciais, chame a função create_dashboard
- Não assuma informações - sempre pergunte diretamente ao usuário

Exemplo de fluxo:
1. "Olá! Vou ajudar você a criar um dashboard no Grafana. Qual será o título do seu dashboard?"
2. "Perfeito! Qual fonte de dados você está usando? (ex: Prometheus, InfluxDB, Graphite...)"
3. "Ótimo! Agora me conte quais métricas você gostaria de visualizar no dashboard?"
4. [Quando tiver tudo] Chamar create_dashboard`;
  }

  async processChat(messages, dashboardContext = {}) {
    try {
      const allMessages = [
        { role: 'system', content: this.systemPrompt },
        ...messages
      ];

      const completion = await this.client.chat.completions.create({
        model: 'gpt-5-nano',
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
        max_tokens: 500
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
      throw new Error(`Erro na comunicação com IA: ${error.message}`);
    }
  }
}