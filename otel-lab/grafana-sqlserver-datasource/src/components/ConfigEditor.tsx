import React, { ChangeEvent } from 'react';
import { DataSourcePluginOptionsEditorProps, GrafanaTheme2 } from '@grafana/data';
import { InlineField, Input, SecretInput, Select, FieldSet, useStyles2 } from '@grafana/ui';
import { css } from '@emotion/css';
import { SQLServerDataSourceOptions, SQLServerSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<SQLServerDataSourceOptions, SQLServerSecureJsonData> {}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    max-width: 600px;
  `,
  fieldSet: css`
    margin-bottom: ${theme.spacing(2)};
  `,
});

const encryptOptions = [
  { label: 'Disable', value: 'disable' },
  { label: 'False (TLS without verification)', value: 'false' },
  { label: 'True (TLS with verification)', value: 'true' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const styles = useStyles2(getStyles);

  const onJsonDataChange = <T extends keyof SQLServerDataSourceOptions>(
    key: T,
    value: SQLServerDataSourceOptions[T]
  ) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, [key]: value },
    });
  };

  const onSecureJsonDataChange = (key: keyof SQLServerSecureJsonData, value: string) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, [key]: value },
    });
  };

  const onResetPassword = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, password: false },
      secureJsonData: { ...secureJsonData, password: '' },
    });
  };

  return (
    <div className={styles.container}>
      <FieldSet label="SQL Server Connection" className={styles.fieldSet}>
        <InlineField label="Host" labelWidth={14} tooltip="SQL Server hostname or IP address">
          <Input
            width={40}
            value={jsonData.host || ''}
            placeholder="localhost"
            onChange={(e: ChangeEvent<HTMLInputElement>) => onJsonDataChange('host', e.target.value)}
          />
        </InlineField>

        <InlineField label="Port" labelWidth={14} tooltip="SQL Server port (default: 1433)">
          <Input
            width={10}
            type="number"
            value={jsonData.port || 1433}
            onChange={(e: ChangeEvent<HTMLInputElement>) => onJsonDataChange('port', parseInt(e.target.value, 10) || 1433)}
          />
        </InlineField>

        <InlineField label="Database" labelWidth={14} tooltip="Database name">
          <Input
            width={40}
            value={jsonData.database || ''}
            placeholder="master"
            onChange={(e: ChangeEvent<HTMLInputElement>) => onJsonDataChange('database', e.target.value)}
          />
        </InlineField>

        <InlineField label="User" labelWidth={14} tooltip="SQL Server username">
          <Input
            width={40}
            value={jsonData.user || ''}
            placeholder="sa"
            onChange={(e: ChangeEvent<HTMLInputElement>) => onJsonDataChange('user', e.target.value)}
          />
        </InlineField>

        <InlineField label="Password" labelWidth={14} tooltip="SQL Server password">
          <SecretInput
            width={40}
            isConfigured={secureJsonFields?.password || false}
            value={secureJsonData?.password || ''}
            placeholder="Enter password"
            onReset={onResetPassword}
            onChange={(e: ChangeEvent<HTMLInputElement>) => onSecureJsonDataChange('password', e.target.value)}
          />
        </InlineField>

        <InlineField label="Encrypt" labelWidth={14} tooltip="TLS/SSL encryption mode">
          <Select
            width={40}
            options={encryptOptions}
            value={jsonData.encrypt || 'disable'}
            onChange={(v) => onJsonDataChange('encrypt', v.value || 'disable')}
          />
        </InlineField>

        <InlineField label="Schema" labelWidth={14} tooltip="Database schema to query tables from (default: dbo)">
          <Input
            width={40}
            value={jsonData.schema || ''}
            placeholder="dbo"
            onChange={(e: ChangeEvent<HTMLInputElement>) => onJsonDataChange('schema', e.target.value)}
          />
        </InlineField>
      </FieldSet>
    </div>
  );
}
