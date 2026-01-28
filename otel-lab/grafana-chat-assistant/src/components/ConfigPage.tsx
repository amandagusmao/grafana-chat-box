import React, { useState, useEffect } from 'react';
import { AppRootProps, GrafanaTheme2, SelectableValue } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { Button, Input, Field, FieldSet, SecretInput, Alert, useStyles2, Select } from '@grafana/ui';
import { css } from '@emotion/css';

interface Props extends AppRootProps {}

interface DatasourceInfo {
  uid: string;
  name: string;
  type: string;
  isDefault: boolean;
}

interface DatasourcesResponse {
  prometheus: DatasourceInfo[];
  loki: DatasourceInfo[];
  tempo: DatasourceInfo[];
  other: DatasourceInfo[];
}

interface PluginSettings {
  jsonData: {
    grafanaUrl?: string;
    aiEndpoint?: string;
    aiModel?: string;
    defaultPrometheusUid?: string;
    defaultLokiUid?: string;
    defaultTempoUid?: string;
  };
  secureJsonFields: {
    identificador?: boolean;
    senha?: boolean;
    grafanaToken?: boolean;
  };
}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    max-width: 700px;
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
  `,
  selectContainer: css`
    display: flex;
    flex-direction: column;
    gap: ${theme.spacing(1)};
  `,
});

export const ConfigPage: React.FC<Props> = () => {
  const styles = useStyles2(getStyles);
  const [settings, setSettings] = useState<PluginSettings | null>(null);
  const [grafanaUrl, setGrafanaUrl] = useState('');
  const [identificador, setIdentificador] = useState('');
  const [senha, setSenha] = useState('');
  const [grafanaToken, setGrafanaToken] = useState('');
  const [aiEndpoint, setAiEndpoint] = useState('');
  const [aiModel, setAiModel] = useState('');
  const [isIdentificadorSet, setIsIdentificadorSet] = useState(false);
  const [isSenhaSet, setIsSenhaSet] = useState(false);
  const [isGrafanaTokenSet, setIsGrafanaTokenSet] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Datasource configuration
  const [datasources, setDatasources] = useState<DatasourcesResponse | null>(null);
  const [loadingDatasources, setLoadingDatasources] = useState(false);
  const [defaultPrometheusUid, setDefaultPrometheusUid] = useState<string>('');
  const [defaultLokiUid, setDefaultLokiUid] = useState<string>('');
  const [defaultTempoUid, setDefaultTempoUid] = useState<string>('');

  useEffect(() => {
    loadSettings();
  }, []);

  useEffect(() => {
    // Load datasources when grafana token is configured
    if (isGrafanaTokenSet || grafanaToken) {
      loadDatasources();
    }
  }, [isGrafanaTokenSet]);

  const loadSettings = async () => {
    try {
      const response = await getBackendSrv().get('/api/plugins/grafana-chat-assistant/settings');
      setSettings(response);
      setGrafanaUrl(response.jsonData?.grafanaUrl || window.location.origin);
      setAiEndpoint(response.jsonData?.aiEndpoint || '');
      setAiModel(response.jsonData?.aiModel || '');
      setDefaultPrometheusUid(response.jsonData?.defaultPrometheusUid || '');
      setDefaultLokiUid(response.jsonData?.defaultLokiUid || '');
      setDefaultTempoUid(response.jsonData?.defaultTempoUid || '');
      setIsIdentificadorSet(response.secureJsonFields?.identificador || false);
      setIsSenhaSet(response.secureJsonFields?.senha || false);
      setIsGrafanaTokenSet(response.secureJsonFields?.grafanaToken || false);

      // Load datasources if token is configured
      if (response.secureJsonFields?.grafanaToken) {
        loadDatasources();
      }
    } catch (error) {
      console.error('Failed to load settings:', error);
      setGrafanaUrl(window.location.origin);
    }
  };

  const loadDatasources = async () => {
    setLoadingDatasources(true);
    try {
      const response = await getBackendSrv().get('/api/plugins/grafana-chat-assistant/resources/datasources');
      setDatasources(response);
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
          grafanaUrl: grafanaUrl || window.location.origin,
          aiEndpoint: aiEndpoint || '',
          aiModel: aiModel || '',
          defaultPrometheusUid: defaultPrometheusUid || '',
          defaultLokiUid: defaultLokiUid || '',
          defaultTempoUid: defaultTempoUid || '',
        },
        secureJsonData: {},
      };

      // Only send credentials if they were changed
      if (identificador) {
        payload.secureJsonData.identificador = identificador;
      }
      if (senha) {
        payload.secureJsonData.senha = senha;
      }
      if (grafanaToken) {
        payload.secureJsonData.grafanaToken = grafanaToken;
      }

      await getBackendSrv().post('/api/plugins/grafana-chat-assistant/settings', payload);

      setMessage({ type: 'success', text: 'Configurações salvas com sucesso!' });
      setIdentificador('');
      setSenha('');
      setGrafanaToken('');
      loadSettings();
    } catch (error: any) {
      console.error('Failed to save settings:', error);
      setMessage({ type: 'error', text: 'Erro ao salvar configurações: ' + error.message });
    } finally {
      setSaving(false);
    }
  };

  const resetIdentificador = () => {
    setIsIdentificadorSet(false);
    setIdentificador('');
  };

  const resetSenha = () => {
    setIsSenhaSet(false);
    setSenha('');
  };

  const resetGrafanaToken = () => {
    setIsGrafanaTokenSet(false);
    setGrafanaToken('');
  };

  // Convert datasources to SelectableValue format
  const datasourceToOptions = (dsList: DatasourceInfo[] | undefined): Array<SelectableValue<string>> => {
    if (!dsList || dsList.length === 0) {
      return [];
    }
    const options: Array<SelectableValue<string>> = [
      { label: '-- Nenhum selecionado --', value: '' }
    ];
    dsList.forEach(ds => {
      options.push({
        label: ds.isDefault ? `${ds.name} (Grafana Default)` : ds.name,
        value: ds.uid,
        description: `UID: ${ds.uid}`,
      });
    });
    return options;
  };

  const prometheusOptions = datasourceToOptions(datasources?.prometheus);
  const lokiOptions = datasourceToOptions(datasources?.loki);
  const tempoOptions = datasourceToOptions(datasources?.tempo);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2>Configurações do Chat Assistant</h2>
        <p>Configure as credenciais necessárias para o funcionamento do assistente.</p>
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

      <FieldSet label="Credenciais de Autenticação" className={styles.fieldSet}>
        <Field
          label="Identificador"
          description="Identificador para autenticação no serviço de IA"
          required
        >
          <SecretInput
            width={40}
            value={identificador}
            isConfigured={isIdentificadorSet}
            placeholder="seu-identificador"
            onChange={(e) => setIdentificador(e.currentTarget.value)}
            onReset={resetIdentificador}
          />
        </Field>

        <Field
          label="Senha"
          description="Senha para autenticação no serviço de IA"
          required
        >
          <SecretInput
            width={40}
            value={senha}
            isConfigured={isSenhaSet}
            placeholder="sua-senha"
            onChange={(e) => setSenha(e.currentTarget.value)}
            onReset={resetSenha}
          />
        </Field>
      </FieldSet>

      <FieldSet label="Configurações da API" className={styles.fieldSet}>
        <Field
          label="Endpoint da API"
          description="URL base da API de IA. Ex: http://host.docker.internal:4000"
          required
        >
          <Input
            width={40}
            value={aiEndpoint}
            placeholder="http://host.docker.internal:4000"
            onChange={(e) => setAiEndpoint(e.currentTarget.value)}
          />
        </Field>

        <Field
          label="Modelo"
          description="Nome do modelo a ser utilizado (opcional)"
        >
          <Input
            width={40}
            value={aiModel}
            placeholder="gpt-4o-mini"
            onChange={(e) => setAiModel(e.currentTarget.value)}
          />
        </Field>
      </FieldSet>

      <FieldSet label="Grafana API" className={styles.fieldSet}>
        <Field
          label="URL do Grafana"
          description="URL da instância Grafana para criar dashboards. Deixe em branco para usar a URL atual."
        >
          <Input
            width={40}
            value={grafanaUrl}
            placeholder={window.location.origin}
            onChange={(e) => setGrafanaUrl(e.currentTarget.value)}
          />
        </Field>

        <Field
          label="Service Account Token"
          description="Token de uma Service Account com permissão para criar dashboards. Necessário para criação de dashboards e configuração de datasources padrão."
        >
          <SecretInput
            width={40}
            value={grafanaToken}
            isConfigured={isGrafanaTokenSet}
            placeholder="glsa_..."
            onChange={(e) => setGrafanaToken(e.currentTarget.value)}
            onReset={resetGrafanaToken}
          />
        </Field>

        {isGrafanaTokenSet && (
          <div style={{ marginTop: '8px' }}>
            <Button variant="secondary" size="sm" onClick={loadDatasources} disabled={loadingDatasources}>
              {loadingDatasources ? 'Carregando...' : 'Recarregar Datasources'}
            </Button>
          </div>
        )}
      </FieldSet>

      {(isGrafanaTokenSet || datasources) && (
        <FieldSet label="Datasources Padrão para o Chat" className={styles.fieldSet}>
          <p className={styles.infoText}>
            Selecione os datasources padrão que o chat usará ao criar dashboards.
            Se o usuário não especificar qual datasource usar, estes serão utilizados automaticamente.
            O usuário ainda poderá solicitar outros datasources explicitamente.
          </p>

          <Field
            label="Prometheus Padrão"
            description={datasources?.prometheus?.length
              ? `${datasources.prometheus.length} datasource(s) Prometheus disponível(is)`
              : 'Nenhum datasource Prometheus encontrado'}
          >
            <Select
              options={prometheusOptions}
              value={defaultPrometheusUid}
              onChange={(v) => setDefaultPrometheusUid(v?.value || '')}
              placeholder="Selecione o Prometheus padrão"
              isClearable
              isLoading={loadingDatasources}
              width={40}
            />
          </Field>

          <Field
            label="Loki Padrão"
            description={datasources?.loki?.length
              ? `${datasources.loki.length} datasource(s) Loki disponível(is)`
              : 'Nenhum datasource Loki encontrado'}
          >
            <Select
              options={lokiOptions}
              value={defaultLokiUid}
              onChange={(v) => setDefaultLokiUid(v?.value || '')}
              placeholder="Selecione o Loki padrão"
              isClearable
              isLoading={loadingDatasources}
              width={40}
            />
          </Field>

          <Field
            label="Tempo Padrão"
            description={datasources?.tempo?.length
              ? `${datasources.tempo.length} datasource(s) Tempo disponível(is)`
              : 'Nenhum datasource Tempo encontrado'}
          >
            <Select
              options={tempoOptions}
              value={defaultTempoUid}
              onChange={(v) => setDefaultTempoUid(v?.value || '')}
              placeholder="Selecione o Tempo padrão"
              isClearable
              isLoading={loadingDatasources}
              width={40}
            />
          </Field>
        </FieldSet>
      )}

      <div className={styles.buttonContainer}>
        <Button onClick={saveSettings} disabled={saving}>
          {saving ? 'Salvando...' : 'Salvar Configurações'}
        </Button>
      </div>
    </div>
  );
};
