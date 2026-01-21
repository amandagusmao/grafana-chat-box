# Média por Séries Temporais

Plugin de transformação para Grafana que calcula a **média histórica** de dados baseada no dia da semana e horário.

## O que faz?

Este plugin compara seus dados atuais com a média histórica do mesmo dia da semana e horário.

**Exemplo prático:**
- Você está visualizando dados de uma quarta-feira às 15:50
- O plugin calcula a média de todas as quartas-feiras anteriores às 15:50
- Isso permite identificar se o valor atual está acima ou abaixo do esperado

## Quando usar?

- **Análise de volumetria**: Compare o volume atual de transações com a média histórica
- **Detecção de anomalias**: Identifique quando valores estão fora do padrão
- **Planejamento de capacidade**: Entenda padrões de uso por dia da semana
- **Dashboards de monitoramento**: Adicione uma linha de referência baseada em dados históricos

## Configurações

| Opção | Descrição |
|-------|-----------|
| **Period** | Período histórico para calcular a média (15d, 30d, 60d, 90d, 180d, 365d, ou todos os dados) |
| **Granularity** | Intervalo de tempo para agrupar os dados (minuto, 5min, 15min, 30min, hora) |
| **Series Name** | Nome da série gerada (padrão: "Média") |
| **Keep Original** | Manter a série original junto com a média calculada |

## Como usar

1. Abra seu painel no Grafana
2. Vá para a aba **Transform**
3. Clique em **Add transformation**
4. Selecione **Média por Séries Temporais**
5. Configure as opções conforme necessário
6. A nova série de média aparecerá no gráfico

## Exemplo de uso

Se você tem dados de volumetria de uma API:

```
Quarta 08/01 15:50 → 503 requisições
Quarta 15/01 15:50 → 455 requisições
```

Ao visualizar Quarta 22/01 às 15:50, a linha de média mostrará **479** (média das quartas anteriores nesse horário).

## Autor

**Amanda Gusmão**

## Licença

MIT
