# Grafana Chat Assistant Plugin

A Grafana app plugin that integrates AI-powered chat assistance directly into your Grafana dashboard for intelligent observability and dashboard creation.

## 🏗️ Architecture

This repository contains two main components:

### 1. **Backend API** (Root Directory)
Express.js server with OpenAI integration:
- **Natural Language Processing** - Powered by OpenAI GPT
- **Grafana API Integration** - Direct communication with Grafana instance
- **Dashboard Creation** - Automated dashboard generation from chat conversations

### 2. **Grafana Chat Plugin** (`otel-lab/grafana-chat-assistant/`)
TypeScript/React app plugin that integrates directly into Grafana's UI:
- **Chat Interface** - Interactive chat within Grafana
- **Plugin Architecture** - Native Grafana app plugin
- **Real-time Communication** - Direct integration with backend API

### 3. **OpenTelemetry Observability Stack** (`otel-lab/`)
Complete observability environment for testing:
- **OpenTelemetry Collector** - Host metrics collection
- **Prometheus** - Metrics storage and scraping
- **Grafana** - Visualization platform with plugin integration

## 🚀 Quick Start

### Prerequisites
- Docker and Docker Compose
- Node.js 16+ and npm
- OpenAI API key
- Grafana Service Account Token

### 1. Start the Observability Stack
```bash
cd otel-lab
docker-compose up -d
```

### 2. Configure and Start the Backend
```bash
cp .env.example .env
# Edit .env with your API keys
npm install
npm run dev
```

### 3. Access the Services
- **Grafana**: http://localhost:3000 (admin:admin)
- **Prometheus**: http://localhost:9090
- **Backend API**: http://localhost:4000

## 🛠️ Development

### OpenTelemetry Lab + Grafana Plugin
```bash
cd otel-lab/grafana-chat-assistant
npm run build          # Production build
npm run dev             # Development with watch mode
npm run lint            # ESLint check
npm run lint:fix        # ESLint with fixes
npm run reset-grafana   # Rebuild plugin + restart Grafana
npm run status-grafana  # Check plugin status
```

### Backend API
```bash
# Backend (port 4000)
npm run dev
```

## 📋 Configuration

### Environment Variables
Create `.env` in the root directory:
```bash
OPENAI_API_KEY=your_openai_api_key_here
GRAFANA_URL=http://localhost:3000
GRAFANA_TOKEN=your_grafana_service_account_token
PORT=4000
```

### Grafana Service Account
1. Go to Grafana → Administration → Service Accounts
2. Create new service account with Editor role
3. Generate token and add to `.env`

## 🎯 Features

- **Real-time Metrics**: Host system monitoring with OpenTelemetry
- **AI Dashboard Creation**: Natural language dashboard specifications
- **Grafana Plugin Integration**: Native app plugin within Grafana UI
- **Backend API**: Express.js server with OpenAI integration
- **Docker Support**: Complete containerized development environment
- **Windows Compatible**: No user restrictions in Docker setup

## 📁 Project Structure

```
├── README.md                          # This file
├── .gitignore                         # Git ignore rules
├── server.js                          # Backend API entry point
├── package.json                       # Backend dependencies
├── .env.example                       # Environment template
├── src/                               # Backend source code
│   ├── controllers/                   # API controllers
│   └── services/                      # Business logic services
│
└── otel-lab/                          # Docker-based observability stack
    ├── docker-compose.yml            # Service orchestration
    ├── otel-config.yaml               # OpenTelemetry Collector config
    ├── prometheus.yml                 # Prometheus configuration
    ├── grafana/                       # Grafana provisioning
    └── grafana-chat-assistant/        # Grafana app plugin
        ├── src/                       # TypeScript/React source
        ├── dist/                      # Built plugin assets
        ├── package.json               # Plugin dependencies
        └── webpack.config.js          # Build configuration
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Follow the development commands in the respective package.json files
4. Ensure all tests pass
5. Submit a pull request

## 📄 License

This project is open source and available under the [MIT License](LICENSE).

## 👤 Author

**Amanda Gusmão**

---

*Built with OpenTelemetry, Grafana, React, and OpenAI GPT*