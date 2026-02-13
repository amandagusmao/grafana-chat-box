import React, { useState, useEffect } from 'react';
import { AppRootProps, GrafanaTheme2, SelectableValue } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { Button, Input, Field, FieldSet, Alert, useStyles2, Select, SecretInput, TextArea, Collapse } from '@grafana/ui';
import { css } from '@emotion/css';
import { DatasourceInfo } from '../types';

interface Props extends AppRootProps {}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    max-width: 900px;
    padding: ${theme.spacing(3)};
  `,
  header: css`
    margin-bottom: ${theme.spacing(3)};
  `,
  fieldSet: css`
    margin-bottom: ${theme.spacing(3)};
  `,
  buttonContainer: css`
    display: flex;
    gap: ${theme.spacing(2)};
    margin-top: ${theme.spacing(3)};
  `,
  alert: css`
    margin-bottom: ${theme.spacing(2)};
  `,
  infoText: css`
    font-size: ${theme.typography.bodySmall.fontSize};
    color: ${theme.colors.text.secondary};
    margin-top: ${theme.spacing(0.5)};
    margin-bottom: ${theme.spacing(1)};
  `,
  codeBlock: css`
    background: ${theme.colors.background.secondary};
    padding: ${theme.spacing(2)};
    border-radius: ${theme.shape.radius.default};
    font-family: monospace;
    font-size: ${theme.typography.bodySmall.fontSize};
    white-space: pre-wrap;
    margin-top: ${theme.spacing(1)};
    margin-bottom: ${theme.spacing(2)};
  `,
  threeColumns: css`
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: ${theme.spacing(2)};
    @media (max-width: 768px) {
      grid-template-columns: 1fr;
    }
  `,
});

// Supported datasource types (SQL + Datalake)
const SUPPORTED_DATASOURCE_TYPES = [
  'mssql',
  'mysql',
  'postgres',
  'grafana-postgresql-datasource',
  'grafana-mssql-datasource',
  'grafana-bigquery-datasource',
  'grafana-athena-datasource',
  'grafana-redshift-datasource',
  'grafana-snowflake-datasource',
  'grafana-clickhouse-datasource',
  'doitintl-bigquery-datasource',
  'marcusolsson-json-datasource',
  'yesoreyeram-infinity-datasource',
];

export const ConfigPage: React.FC<Props> = () => {
  const styles = useStyles2(getStyles);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Connection settings
  const [datasourceUid, setDatasourceUid] = useState('');
  const [grafanaUrl, setGrafanaUrl] = useState('');
  const [grafanaToken, setGrafanaToken] = useState('');
  const [isGrafanaTokenSet, setIsGrafanaTokenSet] = useState(false);

  // Query configuration
  const [selectQuery, setSelectQuery] = useState('');
  const [updateTable, setUpdateTable] = useState('');
  const [idColumn, setIdColumn] = useState('id_cadastro');
  const [nameColumn, setNameColumn] = useState('nome');
  const [maintenanceColumn, setMaintenanceColumn] = useState('manutencao');

  // Audit configuration
  const [auditTable, setAuditTable] = useState('');

  // Access control
  const [allowedOrgId, setAllowedOrgId] = useState('');

  // Datasources
  const [datasources, setDatasources] = useState<DatasourceInfo[]>([]);
  const [loadingDatasources, setLoadingDatasources] = useState(false);

  // UI State
  const [showHelp, setShowHelp] = useState(false);
  const [showAuditHelp, setShowAuditHelp] = useState(false);

  useEffect(() => {
    loadSettings();
    loadDatasources();
  }, []);

  const loadSettings = async () => {
    try {
      const response = await getBackendSrv().get('/api/plugins/grafana-maintenance-manager/settings');
      const jsonData = response.jsonData || {};

      setDatasourceUid(jsonData.datasourceUid || '');
      setGrafanaUrl(jsonData.grafanaUrl || window.location.origin);
      setSelectQuery(jsonData.selectQuery || '');
      setUpdateTable(jsonData.updateTable || '');
      setIdColumn(jsonData.idColumn || 'id_cadastro');
      setNameColumn(jsonData.nameColumn || 'nome');
      setMaintenanceColumn(jsonData.maintenanceColumn || 'manutencao');
      setAuditTable(jsonData.auditTable || '');
      setAllowedOrgId(jsonData.allowedOrgId || '');
      setIsGrafanaTokenSet(response.secureJsonFields?.grafanaToken || false);
    } catch (error) {
      console.error('Failed to load settings:', error);
      setGrafanaUrl(window.location.origin);
    }
  };

  const loadDatasources = async () => {
    setLoadingDatasources(true);
    try {
      const response = await getBackendSrv().get('/api/datasources');
      const allDatasources: DatasourceInfo[] = response.map((ds: any) => ({
        uid: ds.uid,
        name: ds.name,
        type: ds.type,
        isDefault: ds.isDefault || false,
      }));
      setDatasources(allDatasources);
    } catch (error) {
      console.error('Failed to load datasources:', error);
    } finally {
      setLoadingDatasources(false);
    }
  };

  const saveSettings = async () => {
    setSaving(true);
    setMessage(null);

    try {
      const payload: any = {
        enabled: true,
        pinned: true,
        jsonData: {
          datasourceUid,
          grafanaUrl: grafanaUrl || window.location.origin,
          selectQuery,
          updateTable,
          idColumn: idColumn || 'id_cadastro',
          nameColumn: nameColumn || 'nome',
          maintenanceColumn: maintenanceColumn || 'manutencao',
          auditTable,
          allowedOrgId,
        },
        secureJsonData: {},
      };

      if (grafanaToken) {
        payload.secureJsonData.grafanaToken = grafanaToken;
      }

      await getBackendSrv().post('/api/plugins/grafana-maintenance-manager/settings', payload);

      setMessage({ type: 'success', text: 'Configuracoes salvas com sucesso! Reinicie o plugin para aplicar as alteracoes.' });
      setGrafanaToken('');
      loadSettings();
    } catch (error: any) {
      console.error('Failed to save settings:', error);
      setMessage({ type: 'error', text: 'Erro ao salvar configuracoes: ' + error.message });
    } finally {
      setSaving(false);
    }
  };

  const resetGrafanaToken = () => {
    setIsGrafanaTokenSet(false);
    setGrafanaToken('');
  };

  // Filter datasources
  const filteredDatasources = datasources.filter(ds =>
    SUPPORTED_DATASOURCE_TYPES.includes(ds.type)
  );

  const datasourceOptions: Array<SelectableValue<string>> = [
    { label: '-- Selecione um datasource --', value: '' },
    ...filteredDatasources.map(ds => ({
      label: `${ds.name} (${ds.type})`,
      value: ds.uid,
      description: `UID: ${ds.uid}`,
    })),
  ];

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2>Configuracoes do Maintenance Manager</h2>
        <p>Configure a conexao com o banco de dados e as queries de consulta.</p>
      </div>

      {message && (
        <Alert
          severity={message.type}
          title={message.type === 'success' ? 'Sucesso' : 'Erro'}
          className={styles.alert}
        >
          {message.text}
        </Alert>
      )}

      <FieldSet label="Conexao" className={styles.fieldSet}>
        <Field
          label="URL do Grafana"
          description="URL da instancia Grafana. Deixe em branco para usar a URL atual."
        >
          <Input
            width={50}
            value={grafanaUrl}
            placeholder={window.location.origin}
            onChange={(e) => setGrafanaUrl(e.currentTarget.value)}
          />
        </Field>

        <Field
          label="Service Account Token"
          description="Token de uma Service Account com permissao para executar queries no datasource."
          required
        >
          <SecretInput
            width={50}
            value={grafanaToken}
            isConfigured={isGrafanaTokenSet}
            placeholder="glsa_..."
            onChange={(e) => setGrafanaToken(e.currentTarget.value)}
            onReset={resetGrafanaToken}
          />
        </Field>

        <Field
          label="Datasource"
          description="Selecione o datasource SQL que sera usado para as queries."
          required
        >
          <Select
            options={datasourceOptions}
            value={datasourceUid}
            onChange={(v) => setDatasourceUid(v?.value || '')}
            placeholder="Selecione o datasource"
            isClearable
            isLoading={loadingDatasources}
            width={50}
          />
        </Field>

        <Button variant="secondary" size="sm" onClick={loadDatasources} disabled={loadingDatasources}>
          {loadingDatasources ? 'Carregando...' : 'Recarregar Datasources'}
        </Button>
      </FieldSet>

      <FieldSet label="Query de Consulta (SELECT)" className={styles.fieldSet}>
        <Collapse label="Ver instrucoes e exemplo" isOpen={showHelp} onToggle={() => setShowHelp(!showHelp)}>
          <p className={styles.infoText}>
            Configure a query SELECT que sera usada para buscar os registros. A query pode incluir JOINs e colunas calculadas.
            O sistema ira adicionar automaticamente a clausula WHERE com os filtros de busca.
          </p>
          <p className={styles.infoText}>
            <strong>Importante:</strong> Se usar JOINs, configure as colunas com o alias da tabela (ex: sc.id_cadastro) para evitar erros de ambiguidade.
          </p>
          <div className={styles.codeBlock}>{`-- Exemplo de query com JOIN:
SELECT TOP 500
  sc.id_cadastro,
  sc.nome + ' - ' + s.nome as nome,
  sc.manutencao
FROM [servico].[TBL_ServicoHasCadastro] sc
INNER JOIN [servico].[TBL_Servico] s ON s.id = sc.id_servico

-- Configuracao de colunas para este exemplo:
-- Coluna de ID: sc.id_cadastro
-- Coluna de Nome: sc.nome
-- Coluna de Manutencao: sc.manutencao`}</div>
        </Collapse>

        <Field
          label="Query SELECT"
          description="Query SQL para buscar os registros. Nao inclua WHERE - ele sera adicionado automaticamente."
          required
        >
          <TextArea
            value={selectQuery}
            onChange={(e) => setSelectQuery(e.currentTarget.value)}
            rows={8}
            placeholder={`SELECT TOP 500
  sc.id_cadastro,
  sc.nome + ' - ' + s.nome as nome,
  sc.manutencao
FROM [servico].[TBL_ServicoHasCadastro] sc
INNER JOIN [servico].[TBL_Servico] s ON s.id = sc.id_servico`}
          />
        </Field>
      </FieldSet>

      <FieldSet label="Configuracao de Colunas e UPDATE" className={styles.fieldSet}>
        <Field
          label="Tabela para UPDATE"
          description="Nome completo da tabela (com schema) usada para atualizar o status de manutencao."
          required
        >
          <Input
            width={50}
            value={updateTable}
            placeholder="[servico].[TBL_ServicoHasCadastro]"
            onChange={(e) => setUpdateTable(e.currentTarget.value)}
          />
        </Field>

        <p className={styles.infoText}>
          Configure os nomes das colunas. Se usar JOINs, inclua o alias da tabela para evitar ambiguidade (ex: sc.id_cadastro).
        </p>

        <div className={styles.threeColumns}>
          <Field
            label="Coluna de ID"
            description="Coluna para busca por ID e WHERE do UPDATE. Use alias se houver JOIN (ex: sc.id_cadastro)."
            required
          >
            <Input
              value={idColumn}
              placeholder="sc.id_cadastro"
              onChange={(e) => setIdColumn(e.currentTarget.value)}
            />
          </Field>

          <Field
            label="Coluna de Nome"
            description="Coluna para busca por texto. Use alias se houver JOIN (ex: sc.nome)."
            required
          >
            <Input
              value={nameColumn}
              placeholder="sc.nome"
              onChange={(e) => setNameColumn(e.currentTarget.value)}
            />
          </Field>

          <Field
            label="Coluna de Manutencao"
            description="Coluna que armazena o status (0/1). Use alias se houver JOIN."
            required
          >
            <Input
              value={maintenanceColumn}
              placeholder="sc.manutencao"
              onChange={(e) => setMaintenanceColumn(e.currentTarget.value)}
            />
          </Field>
        </div>
      </FieldSet>

      <FieldSet label="Auditoria (Opcional)" className={styles.fieldSet}>
        <p className={styles.infoText}>
          Configure uma tabela para registrar todas as alteracoes feitas pelos usuarios.
          A tabela deve ser criada manualmente com a estrutura abaixo.
        </p>

        <Collapse label="Ver estrutura da tabela de auditoria" isOpen={showAuditHelp} onToggle={() => setShowAuditHelp(!showAuditHelp)}>
          <div className={styles.codeBlock}>{`-- SQL Server
CREATE TABLE [schema].[TBL_MaintenanceAudit] (
  id INT IDENTITY(1,1) PRIMARY KEY,
  user_login NVARCHAR(100),
  user_email NVARCHAR(255),
  action NVARCHAR(100),
  record_id NVARCHAR(50),
  record_name NVARCHAR(500),
  old_value BIT,
  new_value BIT,
  timestamp DATETIME DEFAULT GETDATE()
);

-- Criar indice para melhor performance
CREATE INDEX IX_Audit_RecordId ON [schema].[TBL_MaintenanceAudit] (record_id);
CREATE INDEX IX_Audit_Timestamp ON [schema].[TBL_MaintenanceAudit] (timestamp DESC);`}</div>
        </Collapse>

        <Field
          label="Tabela de Auditoria"
          description="Nome completo da tabela de auditoria (com schema). Deixe em branco para desativar."
        >
          <Input
            width={50}
            value={auditTable}
            placeholder="[servico].[TBL_MaintenanceAudit]"
            onChange={(e) => setAuditTable(e.currentTarget.value)}
          />
        </Field>
      </FieldSet>

      <FieldSet label="Controle de Acesso" className={styles.fieldSet}>
        <Field
          label="ID da Organizacao Permitida"
          description="Somente usuarios desta organizacao podem alterar registros. Deixe em branco para permitir todos."
        >
          <Input
            width={20}
            value={allowedOrgId}
            placeholder="1"
            type="number"
            onChange={(e) => setAllowedOrgId(e.currentTarget.value)}
          />
        </Field>
      </FieldSet>

      <div className={styles.buttonContainer}>
        <Button onClick={saveSettings} disabled={saving}>
          {saving ? 'Salvando...' : 'Salvar Configuracoes'}
        </Button>
      </div>
    </div>
  );
};
