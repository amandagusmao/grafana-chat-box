import React, { useState, useEffect } from 'react';
import { AppRootProps, GrafanaTheme2, SelectableValue } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { Button, Input, Field, FieldSet, Alert, useStyles2, Select, SecretInput, TextArea } from '@grafana/ui';
import { css } from '@emotion/css';
import { DatasourceInfo, PluginSettings } from '../types';

interface Props extends AppRootProps {}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    max-width: 800px;
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
  twoColumns: css`
    display: grid;
    grid-template-columns: 1fr 1fr;
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

  // Table configuration
  const [tableName, setTableName] = useState('');
  const [primaryKeyColumn, setPrimaryKeyColumn] = useState('id');
  const [maintenanceColumn, setMaintenanceColumn] = useState('manutencao');
  const [searchColumn, setSearchColumn] = useState('');
  const [displayNameColumn, setDisplayNameColumn] = useState('');
  const [additionalColumns, setAdditionalColumns] = useState('');

  // Access control
  const [allowedOrgId, setAllowedOrgId] = useState('');

  // Datasources
  const [datasources, setDatasources] = useState<DatasourceInfo[]>([]);
  const [loadingDatasources, setLoadingDatasources] = useState(false);

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
      setTableName(jsonData.tableName || '');
      setPrimaryKeyColumn(jsonData.primaryKeyColumn || 'id');
      setMaintenanceColumn(jsonData.maintenanceColumn || 'manutencao');
      setSearchColumn(jsonData.searchColumn || '');
      setDisplayNameColumn(jsonData.displayNameColumn || '');
      setAdditionalColumns(jsonData.additionalColumns || '');
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
          tableName,
          primaryKeyColumn: primaryKeyColumn || 'id',
          maintenanceColumn: maintenanceColumn || 'manutencao',
          searchColumn,
          displayNameColumn,
          additionalColumns,
          allowedOrgId,
        },
        secureJsonData: {},
      };

      if (grafanaToken) {
        payload.secureJsonData.grafanaToken = grafanaToken;
      }

      await getBackendSrv().post('/api/plugins/grafana-maintenance-manager/settings', payload);

      setMessage({ type: 'success', text: 'Configurações salvas com sucesso! Reinicie o plugin para aplicar as alterações.' });
      setGrafanaToken('');
      loadSettings();
    } catch (error: any) {
      console.error('Failed to save settings:', error);
      setMessage({ type: 'error', text: 'Erro ao salvar configurações: ' + error.message });
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
        <h2>Configurações do Maintenance Manager</h2>
        <p>Configure a conexão com o banco de dados e a estrutura da tabela.</p>
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

      <FieldSet label="Conexão" className={styles.fieldSet}>
        <Field
          label="URL do Grafana"
          description="URL da instância Grafana. Deixe em branco para usar a URL atual."
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
          description="Token de uma Service Account com permissão para executar queries no datasource."
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
          description="Selecione o datasource que contém a tabela. Suporta SQL Server, MySQL, PostgreSQL, BigQuery, Athena, Redshift, Snowflake, ClickHouse e outros."
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

      <FieldSet label="Estrutura da Tabela" className={styles.fieldSet}>
        <p className={styles.infoText}>
          Configure os nomes das colunas da sua tabela. A única coluna obrigatória é a de manutenção.
        </p>

        <Field
          label="Nome da Tabela"
          description="Nome completo da tabela incluindo schema."
          required
        >
          <Input
            width={50}
            value={tableName}
            placeholder="[schema].[tabela] ou schema.tabela"
            onChange={(e) => setTableName(e.currentTarget.value)}
          />
        </Field>

        <div className={styles.twoColumns}>
          <Field
            label="Coluna de Chave Primária"
            description="Nome da coluna que identifica cada registro."
          >
            <Input
              value={primaryKeyColumn}
              placeholder="id"
              onChange={(e) => setPrimaryKeyColumn(e.currentTarget.value)}
            />
          </Field>

          <Field
            label="Coluna de Manutenção"
            description="Nome da coluna que indica o status de manutenção (0/1 ou true/false)."
            required
          >
            <Input
              value={maintenanceColumn}
              placeholder="manutencao"
              onChange={(e) => setMaintenanceColumn(e.currentTarget.value)}
            />
          </Field>
        </div>

        <div className={styles.twoColumns}>
          <Field
            label="Coluna de Busca (ID)"
            description="Coluna usada para buscar por ID (ex: id_cadastro)."
          >
            <Input
              value={searchColumn}
              placeholder="id_cadastro"
              onChange={(e) => setSearchColumn(e.currentTarget.value)}
            />
          </Field>

          <Field
            label="Coluna de Nome/Descrição"
            description="Coluna usada para exibir o nome do item e buscar por texto."
          >
            <Input
              value={displayNameColumn}
              placeholder="nome"
              onChange={(e) => setDisplayNameColumn(e.currentTarget.value)}
            />
          </Field>
        </div>

        <Field
          label="Colunas Adicionais"
          description="Colunas extras para exibir nos resultados, separadas por vírgula."
        >
          <Input
            width={50}
            value={additionalColumns}
            placeholder="id_servico, ativo, id_empreendimento"
            onChange={(e) => setAdditionalColumns(e.currentTarget.value)}
          />
        </Field>
      </FieldSet>

      <FieldSet label="Controle de Acesso" className={styles.fieldSet}>
        <Field
          label="ID da Organização Permitida"
          description="Somente usuários desta organização podem alterar registros. Deixe em branco para permitir todos."
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
          {saving ? 'Salvando...' : 'Salvar Configurações'}
        </Button>
      </div>
    </div>
  );
};
