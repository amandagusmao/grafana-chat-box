# Cálculos por Séries Temporais

Plugin de transformação para Grafana que calcula **média, mediana e recorde histórico** de dados baseados no dia da semana e horário.

## O que faz?

Este plugin compara seus dados atuais com cálculos históricos do mesmo dia da semana e horário. Você pode escolher entre diferentes tipos de cálculo para gerar linhas de referência nos seus gráficos.

**Exemplo prático:**
- Você está visualizando dados de uma quarta-feira às 15:50
- O plugin calcula a média/mediana/recorde de todas as quartas-feiras anteriores às 15:50
- Isso permite identificar se o valor atual está acima ou abaixo do esperado

## Tipos de Cálculo

| Tipo | Descrição |
|------|-----------|
| **Average (Média)** | Calcula a média aritmética dos valores históricos |
| **Record Max (Recorde Máximo)** | Mostra o valor máximo já registrado para aquele horário |
| **Record Min (Recorde Mínimo)** | Mostra o valor mínimo já registrado para aquele horário |
| **Median (Mediana)** | Calcula a mediana dos valores históricos (valor central) |

## Quando usar?

- **Análise de volumetria**: Compare o volume atual de transações com a média/mediana histórica
- **Detecção de anomalias**: Identifique quando valores estão fora do padrão usando recordes
- **Planejamento de capacidade**: Entenda padrões de uso por dia da semana com a mediana
- **Dashboards de monitoramento**: Adicione linhas de referência baseadas em dados históricos
- **Análise de picos**: Use o recorde máximo para visualizar os limites históricos

## Configurações

| Opção | Descrição |
|-------|-----------|
| **Source Series** | Série específica para usar no cálculo (ou todas as séries) |
| **Calculation** | Tipo de cálculo: Média, Recorde Máximo, Recorde Mínimo ou Mediana |
| **Period** | Período histórico para calcular (15d, 30d, 60d, 90d, 180d, 365d, ou todos os dados) |
| **Granularity** | Intervalo de tempo para agrupar os dados (minuto, 5min, 15min, 30min, hora) |
| **Series Name** | Nome personalizado da série gerada |
| **Keep Original** | Manter a série original junto com a série calculada |

## Como usar

1. Abra seu painel no Grafana
2. Vá para a aba **Transform**
3. Clique em **Add transformation**
4. Selecione **Cálculos por Séries Temporais**
5. Escolha a **Source Series** (opcional)
6. Selecione o tipo de **Calculation** desejado
7. Configure as demais opções conforme necessário
8. A nova série aparecerá no gráfico

## Exemplos de uso

### Média histórica
Se você tem dados de volumetria de uma API:

```
Quarta 08/01 15:50 → 503 requisições
Quarta 15/01 15:50 → 455 requisições
```

Ao visualizar Quarta 22/01 às 15:50, a linha de média mostrará **479** (média das quartas anteriores nesse horário).

### Recorde máximo
Para monitorar picos de uso:

```
Quarta 08/01 15:50 → 503 requisições
Quarta 15/01 15:50 → 455 requisições
Quarta 22/01 15:50 → 520 requisições (novo recorde!)
```

A linha de recorde máximo mostraria **520** após o novo pico.

### Mediana
Para análise mais robusta a outliers:

```
Quarta 08/01 15:50 → 500 requisições
Quarta 15/01 15:50 → 450 requisições
Quarta 22/01 15:50 → 1200 requisições (pico anômalo)
```

A média seria **716**, mas a mediana seria **500** - mais representativa do comportamento normal.

## Autor

**Amanda Gusmão**

## Versão

1.1.0

## Licença

MIT
