import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface SQLServerQuery extends DataQuery {
  table: string;
  columns: string[];
  filters: Record<string, string>;
  format: 'table' | 'timeseries';
  timeColumn?: string;
  valueColumn?: string;
  rawSQL?: string;
}

export const defaultQuery: Partial<SQLServerQuery> = {
  table: '',
  columns: [],
  filters: {},
  format: 'table',
};

export interface SQLServerDataSourceOptions extends DataSourceJsonData {
  host: string;
  port: number;
  database: string;
  user: string;
  encrypt: string;
  schema: string;
}

export interface SQLServerSecureJsonData {
  password?: string;
}

export interface TableInfo {
  name: string;
  schema: string;
}

export interface ColumnInfo {
  name: string;
  dataType: string;
}

export interface DistinctValue {
  value: string;
  count: number;
}
