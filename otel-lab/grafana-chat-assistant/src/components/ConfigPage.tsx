import React, { useState, useEffect } from 'react';
import { AppRootProps, GrafanaTheme2 } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { Button, Input, Field, FieldSet, SecretInput, Alert, useStyles2 } from '@grafana/ui';
import { css } from '@emotion/css';

interface Props extends AppRootProps {}

interface PluginSettings {
  jsonData: {
    grafanaUrl?: string;
    aiEndpoint?: string;
    aiModel?: string;
  };
  secureJsonFields: {
    openaiApiKey?: boolean;
    grafanaToken?: boolean;
  };
}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    max-width: 600px;
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
});

export const ConfigPage: React.FC<Props> = () => {
  const styles = useStyles2(getStyles);
  const [settings, setSettings] = useState<PluginSettings | null>(null);
  const [grafanaUrl, setGrafanaUrl] = useState('');
  const [openaiApiKey, setOpenaiApiKey] = useState('');
  const [grafanaToken, setGrafanaToken] = useState('');
  const [aiEndpoint, setAiEndpoint] = useState('');
  const [aiModel, setAiModel] = useState('');
  const [isOpenaiKeySet, setIsOpenaiKeySet] = useState(false);
  const [isGrafanaTokenSet, setIsGrafanaTokenSet] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    try {
      const response = await getBackendSrv().get('/api/plugins/grafana-chat-assistant/settings');
      setSettings(response);
      setGrafanaUrl(response.jsonData?.grafanaUrl || window.location.origin);
      setAiEndpoint(response.jsonData?.aiEndpoint || '');
      setAiModel(response.jsonData?.aiModel || '');
      setIsOpenaiKeySet(response.secureJsonFields?.openaiApiKey || false);
      setIsGrafanaTokenSet(response.secureJsonFields?.grafanaToken || false);
    } catch (error) {
      console.error('Failed to load settings:', error);
      setGrafanaUrl(window.location.origin);
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
        },
        secureJsonData: {},
      };

      // Only send API keys if they were changed
      if (openaiApiKey) {
        payload.secureJsonData.openaiApiKey = openaiApiKey;
      }
      if (grafanaToken) {
        payload.secureJsonData.grafanaToken = grafanaToken;
      }

      await getBackendSrv().post('/api/plugins/grafana-chat-assistant/settings', payload);

      setMessage({ type: 'success', text: 'Configurações salvas com sucesso!' });
      setOpenaiApiKey('');
      setGrafanaToken('');
      loadSettings();
    } catch (error: any) {
      console.error('Failed to save settings:', error);
      setMessage({ type: 'error', text: 'Erro ao salvar configurações: ' + error.message });
    } finally {
      setSaving(false);
    }
  };

  const resetOpenaiKey = () => {
    setIsOpenaiKeySet(false);
    setOpenaiApiKey('');
  };

  const resetGrafanaToken = () => {
    setIsGrafanaTokenSet(false);
    setGrafanaToken('');
  };

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

      <FieldSet label="Configurações de IA" className={styles.fieldSet}>
        <Field
          label="API Key"
          description="Sua chave de API do provedor de IA (OpenAI, Azure, ou compatível)"
          required
        >
          <SecretInput
            width={40}
            value={openaiApiKey}
            isConfigured={isOpenaiKeySet}
            placeholder="sk-..."
            onChange={(e) => setOpenaiApiKey(e.currentTarget.value)}
            onReset={resetOpenaiKey}
          />
        </Field>

        <Field
          label="Endpoint da API"
          description="URL base da API (deixe em branco para usar OpenAI padrão). Ex: https://api.openai.com/v1 ou endpoint Azure/custom"
        >
          <Input
            width={40}
            value={aiEndpoint}
            placeholder="https://api.openai.com/v1"
            onChange={(e) => setAiEndpoint(e.currentTarget.value)}
          />
        </Field>

        <Field
          label="Modelo"
          description="Nome do modelo a ser utilizado. Ex: gpt-4o-mini, gpt-4, claude-3-sonnet, etc."
        >
          <Input
            width={40}
            value={aiModel}
            placeholder="gpt-4o-mini"
            onChange={(e) => setAiModel(e.currentTarget.value)}
          />
        </Field>
      </FieldSet>

      <FieldSet label="Grafana API (Opcional)" className={styles.fieldSet}>
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
          description="Token de uma Service Account com permissão para criar dashboards. Necessário apenas para criação automática de dashboards."
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
      </FieldSet>

      <div className={styles.buttonContainer}>
        <Button onClick={saveSettings} disabled={saving}>
          {saving ? 'Salvando...' : 'Salvar Configurações'}
        </Button>
      </div>
    </div>
  );
};
