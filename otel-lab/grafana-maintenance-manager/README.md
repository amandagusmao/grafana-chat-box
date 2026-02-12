# Grafana Maintenance Manager

Plugin Grafana para gerenciar o status de manutenção de serviços em uma tabela SQL.

## Funcionalidades

- **Configuração por Admin**: Selecionar datasource SQL, definir Org permitida, definir tabela
- **Interface de Usuário**: Buscar por nome ou id_cadastro, visualizar status, alterar manutenção
- **Controle de Acesso**: Somente usuários da Org configurada podem fazer alterações

## Instalação

1. Build do plugin:
```bash
npm install
npm run build
```

2. Reinicie o Grafana ou use:
```bash
npm run reset-grafana
```

## Configuração

1. Acesse **Apps > Maintenance Manager > Configuração**
2. Selecione o datasource SQL (MS SQL, MySQL ou PostgreSQL)
3. Configure o ID da Org permitida
4. Defina o nome completo da tabela (ex: `[servico].[TBL_ServicoHasCadastro]`)
5. Salve as configurações

## Uso

1. Acesse **Apps > Maintenance Manager > Gerenciar**
2. Busque por nome ou id_cadastro
3. Visualize os registros e status de manutenção
4. Clique em "Alterar" para toggle do status de manutenção

## Estrutura da Tabela

A tabela SQL deve conter os seguintes campos:
- `id` (int) - Chave primária
- `id_servico` (int) - ID do serviço
- `nome` (varchar) - Nome do serviço
- `id_cadastro` (int) - ID do cadastro
- `ativo` (bit) - Status ativo
- `is_inverso` (bit) - Flag inverso
- `manutencao` (bit) - Status de manutenção (0 = OK, 1 = Em manutenção)
- `id_dynatrace` (varchar) - ID Dynatrace
- `id_empreendimento` (int) - ID do empreendimento

## Permissões

- **Admin**: Pode configurar o plugin
- **Usuários da Org configurada**: Podem alterar o status de manutenção
- **Outros usuários**: Podem apenas visualizar
