import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { SQLServerQuery, SQLServerDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, SQLServerQuery, SQLServerDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
