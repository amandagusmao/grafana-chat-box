import React, { useEffect, useState, useCallback, useMemo } from 'react';
import { QueryEditorProps, SelectableValue, GrafanaTheme2 } from '@grafana/data';
import {
  InlineField,
  Select,
  MultiSelect,
  InlineFieldRow,
  RadioButtonGroup,
  useStyles2,
  Alert,
  Spinner,
} from '@grafana/ui';
import { css } from '@emotion/css';
import { DataSource } from '../datasource';
import { SQLServerQuery, SQLServerDataSourceOptions, TableInfo, ColumnInfo, DistinctValue } from '../types';

type Props = QueryEditorProps<DataSource, SQLServerQuery, SQLServerDataSourceOptions>;

// Date/time data types that should not show filter dropdowns
const DATE_TIME_TYPES = [
  'datetime',
  'datetime2',
  'date',
  'time',
  'smalldatetime',
  'datetimeoffset',
  'timestamp',
];

const isDateTimeColumn = (dataType: string): boolean => {
  return DATE_TIME_TYPES.includes(dataType.toLowerCase());
};

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    display: flex;
    flex-direction: column;
    gap: ${theme.spacing(1)};
  `,
  stepSection: css`
    margin-top: ${theme.spacing(1)};
    padding: ${theme.spacing(2)};
    background: ${theme.colors.background.secondary};
    border-radius: ${theme.shape.borderRadius(1)};
    border-left: 3px solid ${theme.colors.primary.main};
  `,
  stepTitle: css`
    font-weight: ${theme.typography.fontWeightMedium};
    margin-bottom: ${theme.spacing(1)};
    display: flex;
    align-items: center;
    gap: ${theme.spacing(1)};
    color: ${theme.colors.text.primary};
  `,
  stepNumber: css`
    background: ${theme.colors.primary.main};
    color: ${theme.colors.primary.contrastText};
    border-radius: 50%;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: bold;
  `,
  filterGrid: css`
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: ${theme.spacing(1)};
  `,
  infoText: css`
    color: ${theme.colors.text.secondary};
    font-size: ${theme.typography.bodySmall.fontSize};
    margin-top: ${theme.spacing(0.5)};
    margin-bottom: ${theme.spacing(1)};
  `,
  disabledStep: css`
    opacity: 0.5;
    pointer-events: none;
  `,
});

const formatOptions: Array<SelectableValue<string>> = [
  { label: 'Table', value: 'table' },
  { label: 'Time Series', value: 'timeseries' },
];

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const styles = useStyles2(getStyles);

  // State for loaded data
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [columns, setColumns] = useState<ColumnInfo[]>([]);
  const [columnValues, setColumnValues] = useState<Record<string, DistinctValue[]>>({});
  const [loading, setLoading] = useState({
    tables: false,
    columns: false,
    values: {} as Record<string, boolean>,
  });
  const [error, setError] = useState<string | null>(null);

  // Get schema from datasource settings
  const schema = datasource.getSchema();

  // Load tables on mount
  useEffect(() => {
    const loadTables = async () => {
      setLoading((prev) => ({ ...prev, tables: true }));
      setError(null);
      try {
        const data = await datasource.getTables();
        setTables(data || []);
      } catch (err) {
        setError(`Failed to load tables: ${err}`);
        setTables([]);
      } finally {
        setLoading((prev) => ({ ...prev, tables: false }));
      }
    };
    loadTables();
  }, [datasource]);

  // Load columns when table changes
  useEffect(() => {
    if (!query.table) {
      setColumns([]);
      setColumnValues({});
      return;
    }

    const loadColumns = async () => {
      setLoading((prev) => ({ ...prev, columns: true }));
      try {
        const data = await datasource.getColumns(query.table);
        setColumns(data || []);
        setColumnValues({});
      } catch (err) {
        setError(`Failed to load columns: ${err}`);
        setColumns([]);
      } finally {
        setLoading((prev) => ({ ...prev, columns: false }));
      }
    };
    loadColumns();
  }, [query.table, datasource]);

  // Load distinct values for a column
  const loadColumnValues = useCallback(
    async (column: string) => {
      if (!query.table || columnValues[column]) {
        return;
      }

      setLoading((prev) => ({ ...prev, values: { ...prev.values, [column]: true } }));
      try {
        const data = await datasource.getDistinctValues(query.table, column);
        setColumnValues((prev) => ({ ...prev, [column]: data || [] }));
      } catch (err) {
        console.error(`Failed to load values for ${column}:`, err);
      } finally {
        setLoading((prev) => ({ ...prev, values: { ...prev.values, [column]: false } }));
      }
    },
    [query.table, columnValues, datasource]
  );

  // Convert to SelectableValue options
  const tableOptions: Array<SelectableValue<string>> = tables.map((t) => ({
    label: `${t.schema || schema}.${t.name}`,
    value: t.name,
  }));

  const columnOptions: Array<SelectableValue<string>> = columns.map((c) => ({
    label: `${c.name} (${c.dataType})`,
    value: c.name,
  }));

  const getValueOptions = (column: string): Array<SelectableValue<string>> => {
    return (columnValues[column] || []).map((v) => ({
      label: `${v.value} (${v.count})`,
      value: v.value,
    }));
  };

  // Filter columns that are not date/time types for the filters section
  const filterableColumns = useMemo(() => {
    return columns.filter((c) => !isDateTimeColumn(c.dataType));
  }, [columns]);

  // Check if user has selected columns (to show step 3)
  const hasSelectedColumns = query.columns && query.columns.length > 0;

  // Handlers
  const onTableChange = (selected: SelectableValue<string>) => {
    onChange({ ...query, table: selected?.value || '', columns: [], filters: {} });
    onRunQuery();
  };

  const onColumnsChange = (selected: Array<SelectableValue<string>>) => {
    onChange({ ...query, columns: selected.map((s) => s.value || ''), filters: {} });
    onRunQuery();
  };

  const onFormatChange = (value: string) => {
    onChange({ ...query, format: value as 'table' | 'timeseries' });
    onRunQuery();
  };

  const onTimeColumnChange = (selected: SelectableValue<string>) => {
    onChange({ ...query, timeColumn: selected?.value });
    onRunQuery();
  };

  const onValueColumnChange = (selected: SelectableValue<string>) => {
    onChange({ ...query, valueColumn: selected?.value });
    onRunQuery();
  };

  const onFilterChange = (column: string, value: SelectableValue<string> | null) => {
    const newFilters = { ...query.filters };
    if (value?.value) {
      newFilters[column] = value.value;
    } else {
      delete newFilters[column];
    }
    onChange({ ...query, filters: newFilters });
    onRunQuery();
  };

  // Count active filters
  const activeFiltersCount = Object.keys(query.filters || {}).length;

  // Get columns available for filters based on selected columns
  const availableFilterColumns = useMemo(() => {
    if (!hasSelectedColumns) {
      return [];
    }
    // Only show filters for selected columns that are not date/time types
    return filterableColumns.filter((c) => query.columns?.includes(c.name));
  }, [filterableColumns, hasSelectedColumns, query.columns]);

  return (
    <div className={styles.container}>
      {error && (
        <Alert title="Error" severity="error">
          {error}
        </Alert>
      )}

      {/* Step 1: Select Table */}
      <div className={styles.stepSection}>
        <div className={styles.stepTitle}>
          <span className={styles.stepNumber}>1</span>
          Select Table
          {loading.tables && <Spinner size={14} />}
        </div>
        <InlineFieldRow>
          <InlineField label="Table" labelWidth={14} tooltip={`Select a table from ${schema} schema`}>
            <Select
              width={40}
              options={tableOptions}
              value={query.table}
              onChange={onTableChange}
              isLoading={loading.tables}
              placeholder="Select table..."
              isClearable
            />
          </InlineField>

          <InlineField label="Format" labelWidth={10}>
            <RadioButtonGroup options={formatOptions} value={query.format || 'table'} onChange={onFormatChange} />
          </InlineField>
        </InlineFieldRow>
      </div>

      {/* Step 2: Select Columns (only visible after selecting table) */}
      {query.table && (
        <div className={styles.stepSection}>
          <div className={styles.stepTitle}>
            <span className={styles.stepNumber}>2</span>
            Select Columns
            {loading.columns && <Spinner size={14} />}
          </div>
          <p className={styles.infoText}>
            Select which columns to include in the query. Leave empty to select all columns.
          </p>
          <InlineFieldRow>
            <InlineField
              label="Columns"
              labelWidth={14}
              tooltip="Select columns to include (empty = all columns)"
            >
              <MultiSelect
                width={60}
                options={columnOptions}
                value={query.columns?.map((c) => ({ label: c, value: c })) || []}
                onChange={onColumnsChange}
                isLoading={loading.columns}
                placeholder="All columns"
                isClearable
              />
            </InlineField>
          </InlineFieldRow>

          {query.format === 'timeseries' && (
            <InlineFieldRow>
              <InlineField label="Time Column" labelWidth={14} tooltip="Column containing timestamp values">
                <Select
                  width={30}
                  options={columnOptions}
                  value={query.timeColumn}
                  onChange={onTimeColumnChange}
                  placeholder="Select..."
                  isClearable
                />
              </InlineField>
              <InlineField label="Value Column" labelWidth={14} tooltip="Column containing numeric values">
                <Select
                  width={30}
                  options={columnOptions}
                  value={query.valueColumn}
                  onChange={onValueColumnChange}
                  placeholder="Select..."
                  isClearable
                />
              </InlineField>
            </InlineFieldRow>
          )}
        </div>
      )}

      {/* Step 3: Filter Rows (only visible after selecting columns) */}
      {query.table && hasSelectedColumns && availableFilterColumns.length > 0 && (
        <div className={styles.stepSection}>
          <div className={styles.stepTitle}>
            <span className={styles.stepNumber}>3</span>
            Filter Rows
            {activeFiltersCount > 0 && <span style={{ fontWeight: 'normal' }}>({activeFiltersCount} active)</span>}
          </div>
          <p className={styles.infoText}>
            Filter data by selecting specific values. Values are grouped and show occurrence count.
            Date/time columns are filtered automatically via the time range selector.
          </p>
          <div className={styles.filterGrid}>
            {availableFilterColumns.map((col) => (
              <InlineField key={col.name} label={col.name} labelWidth={20}>
                <Select
                  width={25}
                  options={getValueOptions(col.name)}
                  value={query.filters?.[col.name] ? { label: query.filters[col.name], value: query.filters[col.name] } : null}
                  onChange={(v) => onFilterChange(col.name, v)}
                  onOpenMenu={() => loadColumnValues(col.name)}
                  isLoading={loading.values[col.name]}
                  placeholder="Any..."
                  isClearable
                />
              </InlineField>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
