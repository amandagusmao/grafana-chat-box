import express from 'express';
import cors from 'cors';
import dotenv from 'dotenv';
import { ChatController } from './src/controllers/chat.controller.js';

// Load environment variables
dotenv.config();

const app = express();

// Middleware
app.use(express.json({ limit: '10mb' }));
app.use(cors());

// Validate required environment variables
const requiredEnvVars = ['OPENAI_API_KEY', 'GRAFANA_URL', 'GRAFANA_TOKEN'];
const missingVars = requiredEnvVars.filter(varName => !process.env[varName]);

if (missingVars.length > 0) {
  console.error('❌ Missing required environment variables:', missingVars.join(', '));
  console.error('Please check your .env file');
  process.exit(1);
}

// Initialize controllers
const chatController = new ChatController(
  process.env.OPENAI_API_KEY,
  process.env.GRAFANA_URL,
  process.env.GRAFANA_TOKEN
);

// Routes
app.post('/api/chat', (req, res) => chatController.handleChat(req, res));

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({ 
    status: 'o2k', 
    timestamp: new Date().toISOString(),
    services: {
      openai: !!process.env.OPENAI_API_KEY,
      grafana: !!process.env.GRAFANA_URL && !!process.env.GRAFANA_TOKEN
    }
  });
});

// Error handling middleware
app.use((error, req, res, next) => {
  console.error('Unhandled error:', error);
  res.status(500).json({
    error: 'Internal server error',
    message: error.message
  });
});

// 404 handler
app.use((req, res) => {
  res.status(404).json({
    error: 'Route not found',
    availableRoutes: ['/api/chat', '/health']
  });
});

const PORT = process.env.PORT || 4000;

app.listen(PORT, () => {
  console.log(`🚀 Server running on port ${PORT}`);
});
