# Grafana Chat Assistant - Avaliacao de Prontidao para Distribuicao

**Data:** 30/01/2026
**Versao:** 1.0
**Plugin ID:** grafana-chat-assistant

---

## 1. Seguranca

| # | Controle | Status | Descricao |
|---|----------|--------|-----------|
| 1 | Autenticacao obrigatoria | OK | Todos os endpoints exigem usuario autenticado via `X-Grafana-User` / `X-Grafana-Id`. Requisicoes sem autenticacao retornam HTTP 401. |
| 2 | Verificacao de role (Org Role) | OK | Operacoes de escrita (criar/modificar dashboards) verificam se o usuario possui role `Editor` ou `Admin`. Viewers podem apenas consultar. |
| 3 | Controle de acesso por operacao | OK | Whitelist de operacoes de escrita (`create_dashboard`, `search_dashboards`). Tools de leitura (metricas, datasources) sao permitidas para todos os roles autenticados. |
| 4 | Privacidade entre usuarios | OK | Cada requisicao e processada isoladamente. O historico de conversas e armazenado localmente no navegador (`localStorage`) de cada usuario. Nao ha compartilhamento de dados entre sessoes de usuarios diferentes. |
| 5 | Limite de tamanho de requisicao | OK | `http.MaxBytesReader` limita o body a 100KB, prevenindo ataques de exaustao de memoria. |
| 6 | Validacao de entrada | OK | Mensagens limitadas a 2.000 caracteres. Historico truncado para as ultimas 30 mensagens antes de envio a API de IA. |
| 7 | Protecao XSS | OK | HTML e sanitizado via `escapeHtml()` antes de renderizacao. Apenas formatacao segura (negrito, codigo, links http/https) e aplicada apos sanitizacao. |
| 8 | Sanitizacao de erros | OK | Mensagens de erro expostas ao usuario sao genericas. Detalhes tecnicos (stack traces, URLs internas, nomes de servicos) sao registrados apenas em logs do servidor. |
| 9 | Protecao de credenciais SA | OK | Token de Service Account e chave OpenAI sao armazenados em `secureJsonData` (criptografado pelo Grafana). A tool `get_user_permissions` foi removida para evitar exposicao de informacoes internas. |
| 10 | Validacao de queries | OK | `query_validator.go` analisa queries PromQL antes da execucao, bloqueando padroes perigosos ou malformados. |
| 11 | Validacao de parametros de dashboard | OK | `validateDashboardParams` verifica campos obrigatorios (titulo, datasource, paineis, tags), limites (max 20 paineis), e formato antes da criacao. |
| 12 | Imputacao de usuario | OK | Dashboards sao criados usando header `X-Grafana-User` para atribuicao correta de autoria. Permissoes herdadas do papel organizacional sao preservadas. |
| 13 | Sem exposicao de rotas internas | OK | Endpoint `/datasources` exige autenticacao. Nenhum endpoint expoe informacoes de infraestrutura ou configuracao interna. |

---

## 2. Escopo e Etica

| # | Controle | Status | Descricao |
|---|----------|--------|-----------|
| 1 | Restricao de escopo | OK | System prompt define explicitamente que o assistente so responde sobre Grafana, observabilidade, metricas e dashboards. Solicitacoes fora de escopo sao recusadas educadamente. |
| 2 | Protecao anti-jailbreak | OK | Instrucoes no system prompt impedem que o usuario contorne restricoes via engenharia de prompt (role-play, instrucoes de "ignorar regras", etc.). |
| 3 | Protecao de credenciais | OK | O assistente nunca revela chaves de API, tokens, senhas ou informacoes de configuracao interna, mesmo se solicitado diretamente pelo usuario. |
| 4 | Protecao do prompt | OK | O assistente nao revela o conteudo do system prompt quando solicitado pelo usuario. |
| 5 | Trilha de auditoria | OK | Todas as operacoes de escrita sao registradas nos logs do Grafana com identificacao do usuario que as realizou (via impersonation). |
| 6 | Atribuicao correta | OK | Dashboards criados sao atribuidos ao usuario solicitante, nao ao Service Account. Isso garante rastreabilidade e responsabilidade. |

---

## 3. Economia de Tokens

| # | Controle | Status | Descricao |
|---|----------|--------|-----------|
| 1 | MaxTokens | OK | Resposta da IA limitada a 1.500 tokens por requisicao, otimizando custo sem prejudicar qualidade. |
| 2 | Truncamento de historico | OK | Apenas as ultimas 30 mensagens sao enviadas a API de IA, evitando acumulo excessivo de contexto. |
| 3 | Concisao nas respostas | OK | System prompt instrui respostas diretas e objetivas, sem repeticoes ou preambulos desnecessarios. |
| 4 | Cache de metricas | OK | `DiscoveryService` possui cache de metricas com TTL, evitando consultas repetidas ao Prometheus. |
| 5 | Limite de resultados | OK | Buscas de metricas e dashboards retornam no maximo 50 resultados, prevenindo payloads excessivos. |
| 6 | Limite de conversas | OK | Maximo de 50 conversas armazenadas no `localStorage`. Conversas excedentes sao removidas automaticamente (FIFO por data). |

---

## 4. Qualidade dos Dashboards

| # | Controle | Status | Descricao |
|---|----------|--------|-----------|
| 1 | Metricas reais | OK | Dashboards sao criados apenas com metricas verificadas no ambiente. O assistente usa `search_metrics` para validar existencia antes de criar paineis. |
| 2 | Metodologias de observabilidade | OK | System prompt inclui diretrizes para Golden Signals, RED Method e USE Method, com tipos de visualizacao recomendados para cada metrica. |
| 3 | Tags automaticas | OK | Backend `inferTags()` garante tags pertinentes mesmo quando a IA nao as fornece. Tags sao inferidas a partir do titulo, queries e tipos de painel. Tag `ai-generated` adicionada automaticamente. |
| 4 | Resolucao de colisao de nomes | OK | `resolveUniqueTitle()` verifica existencia do titulo antes da criacao. Se ja existir, adiciona sufixo numerico (2-10) ou timestamp. |
| 5 | Limite de paineis | OK | Maximo de 20 paineis por dashboard, garantindo dashboards com boa performance de renderizacao. |
| 6 | Sem simplificacao | OK | Instrucoes de economia de tokens explicitamente protegem a criacao de dashboards. A IA deve criar dashboards completos e com valor real, nunca versoes simplificadas. |

---

## 5. Arquitetura de Seguranca - Fluxo de Requisicao

```
Usuario (Browser)
    |
    v
[1] Grafana (Autenticacao + Sessao)
    |
    v
[2] Plugin Backend (Verificacao de Role + Validacao de Input)
    |
    v
[3] OpenAI API (System Prompt com Restricoes)
    |
    v
[4] Tool Execution (Whitelist + Validacao de Parametros)
    |
    v
[5] Grafana API (Impersonation do Usuario + Permissoes Herdadas)
```

**Camada 1 - Grafana:** Autentica o usuario e gerencia a sessao. O plugin recebe os headers `X-Grafana-User` e `X-Grafana-Id` com a identidade verificada.

**Camada 2 - Plugin Backend:** Valida tamanho da requisicao (100KB), comprimento de mensagens (2.000 chars), historico (30 mensagens) e role organizacional do usuario.

**Camada 3 - OpenAI API:** System prompt com restricoes de escopo, anti-jailbreak, protecao de credenciais e protecao do prompt. MaxTokens limitado a 1.500.

**Camada 4 - Tool Execution:** Apenas tools registradas sao executaveis. Operacoes de escrita exigem role Editor/Admin. Parametros de dashboard sao validados antes da execucao.

**Camada 5 - Grafana API:** Dashboards sao criados via API do Grafana usando impersonation (`X-Grafana-User`), garantindo atribuicao correta e respeitando permissoes do modelo organizacional.

---

## 6. Matriz de Permissoes por Role

| Funcionalidade | Viewer | Editor | Admin |
|----------------|--------|--------|-------|
| Enviar mensagens no chat | Sim | Sim | Sim |
| Consultar metricas disponiveis | Sim | Sim | Sim |
| Buscar dashboards existentes | Sim | Sim | Sim |
| Listar datasources | Sim | Sim | Sim |
| Tirar duvidas sobre Grafana | Sim | Sim | Sim |
| Criar dashboards | Nao | Sim | Sim |
| Modificar dashboards | Nao | Sim | Sim |
| Configurar o plugin | Nao | Nao | Sim |

---

## 7. Itens Verificados sem Problemas Pendentes

- Nenhuma vulnerabilidade de seguranca conhecida
- Nenhuma exposicao de dados sensiveis
- Nenhum endpoint sem autenticacao
- Nenhuma operacao de escrita sem verificacao de role
- Nenhum risco de token overflow ou exaustao de memoria
- Nenhuma funcionalidade fora de escopo acessivel

---

## 8. Conclusao

O plugin **Grafana Chat Assistant** esta pronto para distribuicao. Todos os controles de seguranca, etica, economia de tokens e qualidade de dashboards foram implementados e verificados. O plugin opera dentro do modelo de seguranca do Grafana, respeita permissoes organizacionais e protege dados sensiveis.

### Recomendacoes para o ambiente de producao:

1. **Service Account:** Criar um Service Account dedicado com role `Editor` para o plugin. Nao usar tokens de Admin.
2. **Monitoramento:** Acompanhar os logs do Grafana para atividades do plugin, especialmente criacao de dashboards.
3. **Rotacao de chaves:** Rotacionar periodicamente a chave da API OpenAI e o token do Service Account.
4. **Onboarding:** Orientar usuarios sobre o escopo do assistente (Grafana e observabilidade) e que dashboards sao criados com as metricas reais do ambiente.
