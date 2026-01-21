import { DataSourceInstanceSettings, CoreApp } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';
import {
  SQLServerQuery,
  SQLServerDataSourceOptions,
  TableInfo,
  ColumnInfo,
  DistinctValue,
  defaultQuery,
} from './types';

export class DataSource extends DataSourceWithBackend<SQLServerQuery, SQLServerDataSourceOptions> {
  settings: DataSourceInstanceSettings<SQLServerDataSourceOptions>;

  constructor(instanceSettings: DataSourceInstanceSettings<SQLServerDataSourceOptions>) {
    super(instanceSettings);
    this.settings = instanceSettings;
  }

  getSchema(): string {
    return this.settings.jsonData?.schema || 'dbo';
  }

  getDefaultQuery(_: CoreApp): Partial<SQLServerQuery> {
    return defaultQuery;
  }

  // Resource API methods for query editor
  async getTables(): Promise<TableInfo[]> {
    return this.getResource<TableInfo[]>('tables');
  }

  async getColumns(table: string): Promise<ColumnInfo[]> {
    return this.getResource<ColumnInfo[]>('columns', { table });
  }

  async getDistinctValues(table: string, column: string): Promise<DistinctValue[]> {
    return this.getResource<DistinctValue[]>('values', { table, column });
  }
}
