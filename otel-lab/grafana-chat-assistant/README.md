# Grafana Chat Assistant

Assistente de IA integrado ao Grafana para tirar dúvidas sobre observabilidade e criar dashboards de forma conversacional.

## O que este plugin faz

O Chat Assistant é um chat inteligente dentro do Grafana, especializado em observabilidade. Ele conhece sua instância — datasources configurados, métricas disponíveis, dashboards existentes — e usa esse contexto para te ajudar de forma prática.

**Importante:** O assistente responde exclusivamente sobre Grafana e observabilidade. Perguntas fora desse escopo serão recusadas educadamente.

## Funcionalidades

### Conversa sobre Grafana e Observabilidade

- Tire dúvidas sobre conceitos de observabilidade (Golden Signals, RED, USE, SLI/SLO)
- Pergunte sobre PromQL, LogQL e TraceQL — sintaxe, funções, boas práticas
- Peça recomendações de painéis e visualizações para diferentes cenários
- Entenda quando usar cada tipo de visualização (timeseries, gauge, stat, heatmap, etc.)

### Descoberta do Ambiente

O assistente consulta sua instância em tempo real:

- **Métricas** — busca métricas Prometheus disponíveis por padrão (ex: `node_`, `http_`, `process_`)
- **Datasources** — lista todos os datasources configurados (Prometheus, Loki, Tempo e outros)
- **Labels Loki** — descobre labels disponíveis nos seus logs
- **Serviços Tempo** — lista serviços disponíveis para consultas de traces
- **Dashboards existentes** — pesquisa dashboards por nome ou tema
- **Seus dashboards** — lista dashboards que você criou ou que foram criados para você

### Criação de Dashboards

Peça um dashboard em linguagem natural e o assistente:

1. Descobre quais métricas existem no seu ambiente
2. Seleciona as métricas mais relevantes para o que você pediu
3. Escolhe os tipos de visualização mais adequados para cada métrica
4. Cria o dashboard no Grafana com painéis configurados, thresholds e organização lógica
5. Gera um link direto para você acessar o dashboard criado

**Recursos da criação:**

- Nome gerado automaticamente se você não especificar um
- Se já existir um dashboard com o mesmo nome, o assistente escolhe outro automaticamente
- Tags relevantes adicionadas com base no conteúdo (cpu, memory, golden-signals, etc.)
- Suporte a variáveis de template para dashboards dinâmicos
- Dashboard criado em nome do usuário que solicitou (para auditoria)

### Consulta de Informações do Usuário

- Consulte suas próprias permissões e role na organização
- Administradores podem consultar informações de outros usuários

## Exemplos de uso

**Descoberta e aprendizado:**
- "Quais métricas de CPU estão disponíveis?"
- "O que é o RED Method e como aplico no Grafana?"
- "Qual a diferença entre rate() e irate() no PromQL?"
- "Quais datasources estão configurados nessa instância?"

**Criação de dashboards:**
- "Crie um dashboard de infraestrutura com CPU, memória e disco"
- "Quero um dashboard com Golden Signals para meus serviços HTTP"
- "Monte um painel mostrando a utilização de rede das últimas 6 horas"
- "Preciso de um dashboard USE Method para monitorar saturação dos hosts"

**Exploração:**
- "Quais dashboards existem sobre network?"
- "Liste os serviços disponíveis no Tempo"
- "Quais labels posso usar para filtrar logs no Loki?"

## Permissões

| Ação | Viewer | Editor | Admin |
|------|--------|--------|-------|
| Conversar e tirar dúvidas | Sim | Sim | Sim |
| Buscar métricas e datasources | Sim | Sim | Sim |
| Pesquisar dashboards | Sim | Sim | Sim |
| Criar dashboards | Não | Sim | Sim |
| Consultar dados de outros usuários | Não | Não | Sim |

## Acesso

Após o plugin estar habilitado, acesse pelo menu lateral do Grafana: **Chat Assistant**.

## Autora

Amanda Gusmão
