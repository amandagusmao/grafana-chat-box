# Grafana Chat Assistant

Um plugin Grafana com backend nativo em Go que adiciona um assistente de IA para auxiliar na criação de dashboards.

## Funcionalidades

- Chat interativo com IA (OpenAI GPT-4o-mini)
- Sugestões inteligentes de métricas e visualizações
- Criação automática de dashboards via conversa
- Backend Go nativo - plugin completamente autônomo
- Suporte a múltiplas plataformas (Linux, Windows, macOS)

## Requisitos

### Para Build
- Node.js 18+
- Go 1.21+
- Mage (Go build tool): `go install github.com/magefile/mage@latest`

### Para Execução
- Grafana 9.0+
- Chave de API da OpenAI

## Instalação

### Opção 1: Build local

1. Clone o repositório e navegue até a pasta do plugin:
```bash
cd otel-lab/grafana-chat-assistant
```

2. Instale as dependências do frontend:
```bash
npm install
```

3. Instale as dependências do backend:
```bash
go mod download
```

4. Build completo (frontend + backend para todas as plataformas):
```bash
npm run build
```

Ou build apenas para uma plataforma específica:
```bash
npm run build:frontend
npm run build:backend:linux    # Linux amd64
npm run build:backend:windows  # Windows amd64
npm run build:backend:darwin   # macOS (amd64 + arm64)
```

5. Copie a pasta `dist/` para o diretório de plugins do Grafana:
```bash
cp -r dist /var/lib/grafana/plugins/grafana-chat-assistant
```

6. Reinicie o Grafana.

### Opção 2: Docker (desenvolvimento)

O plugin já está configurado no docker-compose do projeto:

```bash
cd otel-lab
docker-compose up -d
```

## Configuração

1. Acesse o Grafana e vá para **Administration > Plugins**
2. Encontre "Grafana Chat Assistant" e clique em **Enable**
3. Vá para a página de configuração do plugin
4. Configure:
   - **OpenAI API Key** (obrigatório): Sua chave de API da OpenAI
   - **Grafana URL** (opcional): URL da instância Grafana
   - **Service Account Token** (opcional): Token para criação automática de dashboards

### Criando um Service Account Token

Para habilitar a criação automática de dashboards:

1. Vá em **Administration > Service Accounts**
2. Crie uma nova Service Account
3. Adicione a role "Editor" ou "Admin"
4. Gere um token e copie para as configurações do plugin

## Uso

1. Após habilitar o plugin, acesse pelo menu lateral: **Chat Assistant**
2. Converse naturalmente sobre suas necessidades de observabilidade
3. O assistente irá:
   - Sugerir métricas apropriadas
   - Recomendar tipos de visualização
   - Criar dashboards automaticamente quando solicitado

### Exemplos de Perguntas

- "Preciso de um dashboard para monitorar minha API REST"
- "Quais métricas devo usar para observar um banco PostgreSQL?"
- "Crie um dashboard de performance com métricas do Prometheus"

## Estrutura do Projeto

```
grafana-chat-assistant/
├── src/                    # Frontend (React/TypeScript)
│   ├── components/
│   │   ├── ChatAppPage.tsx # Interface do chat
│   │   └── ConfigPage.tsx  # Página de configuração
│   ├── module.tsx          # Entry point do plugin
│   └── plugin.json         # Manifesto do plugin
├── pkg/                    # Backend (Go)
│   ├── main.go
│   └── plugin/
│       ├── app.go          # Inicialização do app
│       ├── handlers.go     # HTTP handlers
│       ├── openai_service.go
│       └── grafana_service.go
├── dist/                   # Build output
├── go.mod
├── Magefile.go            # Build system Go
└── package.json
```

## Desenvolvimento

### Frontend
```bash
npm run dev  # Watch mode
```

### Backend
```bash
go build -o dist/gpx_grafana-chat-assistant_linux_amd64 ./pkg
```

### Lint
```bash
npm run lint
npm run lint:fix
```

## Configuração do Grafana (Unsigned Plugin)

Para instâncias que não permitem plugins não assinados, adicione ao `grafana.ini`:

```ini
[plugins]
allow_loading_unsigned_plugins = grafana-chat-assistant
```

Ou via variável de ambiente:
```bash
GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=grafana-chat-assistant
```

## Autor

Amanda Gusmão

## Licença

MIT
