import { OpenAIService } from '../services/openai.service.js';
import { GrafanaService } from '../services/grafana.service.js';

export class ChatController {
  constructor(openaiApiKey, grafanaUrl, grafanaToken) {
    this.openaiService = new OpenAIService(openaiApiKey);
    this.grafanaService = new GrafanaService(grafanaUrl, grafanaToken);
  }

  async handleChat(req, res) {
    try {
      console.log('🔍 Received chat request');
      console.log('🔍 Messages count:', req.body.messages?.length);
      console.log('🔍 Last message:', req.body.messages?.[req.body.messages.length - 1]?.content);
      
      const { messages, dashboardContext = {} } = req.body;

      if (!messages || !Array.isArray(messages)) {
        return res.status(400).json({
          error: 'Messages array is required'
        });
      }

      // Process chat with OpenAI
      console.log('🔍 Processing with OpenAI...');
      const aiResponse = await this.openaiService.processChat(messages, dashboardContext);
      console.log('🔍 OpenAI response type:', aiResponse.type);

      // If AI wants to create dashboard, do it
      if (aiResponse.type === 'function_call' && aiResponse.function === 'create_dashboard') {
        try {
          const dashboardResult = await this.grafanaService.createDashboard(aiResponse.data);
          
          return res.json({
            type: 'dashboard_created',
            message: aiResponse.response,
            dashboard: {
              url: `${process.env.GRAFANA_URL}/d/${dashboardResult.uid}`,
              uid: dashboardResult.uid,
              id: dashboardResult.id,
              title: aiResponse.data.title
            },
            success: true
          });
        } catch (dashboardError) {
          return res.json({
            type: 'message',
            message: `Desculpe, ocorreu um erro ao criar o dashboard: ${dashboardError.message}. Você pode tentar novamente ou ajustar as informações.`,
            success: true
          });
        }
      }

      // Regular chat response
      return res.json({
        type: 'message',
        message: aiResponse.response,
        success: true
      });

    } catch (error) {
      console.error('Chat error:', error);
      return res.status(500).json({
        error: `Erro interno do servidor: ${error.message}`,
        success: false
      });
    }
  }
}