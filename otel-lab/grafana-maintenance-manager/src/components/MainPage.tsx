import React, { useState, useEffect } from 'react';
import { AppRootProps, GrafanaTheme2 } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import {
  Button,
  Field,
  Alert,
  useStyles2,
  Badge,
  ConfirmModal,
  LoadingPlaceholder,
  RadioButtonGroup,
  TextArea,
  Icon,
  Tooltip,
  InlineSwitch
} from '@grafana/ui';
import { css } from '@emotion/css';
import { ServiceRecord, PermissionResponse, SearchResponse, UpdateResponse, TableConfig } from '../types';

interface Props extends AppRootProps {}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    padding: ${theme.spacing(3)};
    max-width: 1400px;
  `,
  header: css`
    margin-bottom: ${theme.spacing(2)};
  `,
  searchSection: css`
    background: ${theme.colors.background.secondary};
    padding: ${theme.spacing(2)};
    border-radius: ${theme.shape.radius.default};
    margin-bottom: ${theme.spacing(3)};
  `,
  searchRow: css`
    display: flex;
    gap: ${theme.spacing(2)};
    align-items: flex-start;
    flex-wrap: wrap;
  `,
  searchInput: css`
    flex: 1;
    min-width: 300px;
  `,
  searchButtons: css`
    display: flex;
    gap: ${theme.spacing(1)};
    align-items: center;
    padding-top: ${theme.spacing(3)};
  `,
  searchHelp: css`
    font-size: ${theme.typography.bodySmall.fontSize};
    color: ${theme.colors.text.secondary};
    margin-top: ${theme.spacing(1)};
  `,
  resultsSection: css`
    margin-top: ${theme.spacing(2)};
  `,
  resultsHeader: css`
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: ${theme.spacing(2)};
  `,
  table: css`
    width: 100%;
    border-collapse: collapse;
    background: ${theme.colors.background.primary};
    border-radius: ${theme.shape.radius.default};
    overflow: hidden;
  `,
  tableHeader: css`
    background: ${theme.colors.background.secondary};
    th {
      padding: ${theme.spacing(1.5)} ${theme.spacing(2)};
      text-align: left;
      font-weight: ${theme.typography.fontWeightMedium};
      border-bottom: 1px solid ${theme.colors.border.weak};
    }
  `,
  tableRow: css`
    &:hover {
      background: ${theme.colors.background.secondary};
    }
    td {
      padding: ${theme.spacing(1.5)} ${theme.spacing(2)};
      border-bottom: 1px solid ${theme.colors.border.weak};
      vertical-align: middle;
    }
  `,
  statusCell: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(1)};
  `,
  actionCell: css`
    text-align: right;
    white-space: nowrap;
  `,
  alert: css`
    margin-bottom: ${theme.spacing(2)};
  `,
  noResults: css`
    text-align: center;
    padding: ${theme.spacing(4)};
    color: ${theme.colors.text.secondary};
  `,
  statsBar: css`
    display: flex;
    gap: ${theme.spacing(3)};
    margin-bottom: ${theme.spacing(2)};
    padding: ${theme.spacing(1)} ${theme.spacing(2)};
    background: ${theme.colors.background.secondary};
    border-radius: ${theme.shape.radius.default};
  `,
  statItem: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(0.5)};
    font-size: ${theme.typography.bodySmall.fontSize};
  `,
  bulkActions: css`
    display: flex;
    gap: ${theme.spacing(1)};
    align-items: center;
  `,
});

type SearchMode = 'id' | 'name';

export const MainPage: React.FC<Props> = () => {
  const styles = useStyles2(getStyles);

  // Search state
  const [searchText, setSearchText] = useState('');
  const [searchMode, setSearchMode] = useState<SearchMode>('id');
  const [records, setRecords] = useState<ServiceRecord[]>([]);
  const [tableConfig, setTableConfig] = useState<TableConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [searching, setSearching] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error' | 'warning'; text: string } | null>(null);

  // Permission state
  const [permission, setPermission] = useState<PermissionResponse | null>(null);
  const [loadingPermission, setLoadingPermission] = useState(true);

  // Selected records for bulk actions
  const [selectedIds, setSelectedIds] = useState<Set<any>>(new Set());

  // Modal state
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    record: ServiceRecord | null;
    newStatus: boolean;
    isBulk: boolean;
  }>({ isOpen: false, record: null, newStatus: false, isBulk: false });

  useEffect(() => {
    checkPermission();
  }, []);

  const checkPermission = async () => {
    setLoadingPermission(true);
    try {
      const response = await getBackendSrv().get('/api/plugins/grafana-maintenance-manager/resources/check-permission');
      setPermission(response);
    } catch (error) {
      console.error('Failed to check permission:', error);
      setPermission({
        hasPermission: false,
        currentOrgId: 0,
        allowedOrgId: 0,
        userLogin: '',
        message: 'Erro ao verificar permissões'
      });
    } finally {
      setLoadingPermission(false);
    }
  };

  const parseSearchValues = (text: string): string[] => {
    // Split by comma, newline, semicolon, or space (for multiple IDs)
    return text
      .split(/[,;\n\s]+/)
      .map(v => v.trim())
      .filter(v => v.length > 0);
  };

  const handleSearch = async () => {
    const values = parseSearchValues(searchText);
    if (values.length === 0) {
      setMessage({ type: 'warning', text: 'Informe pelo menos um valor para buscar.' });
      return;
    }

    setSearching(true);
    setMessage(null);
    setSelectedIds(new Set());

    try {
      const response: SearchResponse = await getBackendSrv().post(
        '/api/plugins/grafana-maintenance-manager/resources/search',
        {
          searchValues: values,
          searchByName: searchMode === 'name'
        }
      );

      if (response.success) {
        setRecords(response.records || []);
        setTableConfig(response.config || null);
        if (response.records.length === 0) {
          setMessage({ type: 'warning', text: 'Nenhum registro encontrado.' });
        }
      } else {
        setMessage({ type: 'error', text: response.error || 'Erro ao buscar registros.' });
      }
    } catch (error: any) {
      console.error('Search failed:', error);
      setMessage({ type: 'error', text: 'Erro ao buscar registros: ' + (error.message || error) });
    } finally {
      setSearching(false);
    }
  };

  const handleToggleMaintenance = (record: ServiceRecord) => {
    setConfirmModal({
      isOpen: true,
      record: record,
      newStatus: !record.manutencao,
      isBulk: false
    });
  };

  const handleBulkToggle = (newStatus: boolean) => {
    if (selectedIds.size === 0) return;
    setConfirmModal({
      isOpen: true,
      record: null,
      newStatus: newStatus,
      isBulk: true
    });
  };

  const confirmToggle = async () => {
    setLoading(true);
    setMessage(null);

    try {
      if (confirmModal.isBulk) {
        // Bulk update
        const idsToUpdate = Array.from(selectedIds);
        let successCount = 0;
        let errorCount = 0;

        for (const id of idsToUpdate) {
          try {
            const response: UpdateResponse = await getBackendSrv().post(
              '/api/plugins/grafana-maintenance-manager/resources/update',
              { id, manutencao: confirmModal.newStatus }
            );
            if (response.success) {
              successCount++;
            } else {
              errorCount++;
            }
          } catch {
            errorCount++;
          }
        }

        if (successCount > 0) {
          setRecords(records.map(r =>
            selectedIds.has(r.id) ? { ...r, manutencao: confirmModal.newStatus } : r
          ));
          setSelectedIds(new Set());
        }

        if (errorCount > 0) {
          setMessage({
            type: 'warning',
            text: `${successCount} registro(s) atualizado(s), ${errorCount} erro(s).`
          });
        } else {
          setMessage({
            type: 'success',
            text: `${successCount} registro(s) atualizado(s) com sucesso.`
          });
        }
      } else if (confirmModal.record) {
        // Single update
        const response: UpdateResponse = await getBackendSrv().post(
          '/api/plugins/grafana-maintenance-manager/resources/update',
          { id: confirmModal.record.id, manutencao: confirmModal.newStatus }
        );

        if (response.success) {
          setMessage({
            type: 'success',
            text: `Status alterado para ${confirmModal.newStatus ? 'Em Manutenção' : 'Normal'}.`
          });
          setRecords(records.map(r =>
            r.id === confirmModal.record!.id ? { ...r, manutencao: confirmModal.newStatus } : r
          ));
        } else {
          setMessage({ type: 'error', text: response.error || 'Erro ao atualizar registro.' });
        }
      }
    } catch (error: any) {
      console.error('Update failed:', error);
      setMessage({ type: 'error', text: 'Erro ao atualizar: ' + (error.message || error) });
    } finally {
      setLoading(false);
      setConfirmModal({ isOpen: false, record: null, newStatus: false, isBulk: false });
    }
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === records.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(records.map(r => r.id)));
    }
  };

  const toggleSelect = (id: any) => {
    const newSelected = new Set(selectedIds);
    if (newSelected.has(id)) {
      newSelected.delete(id);
    } else {
      newSelected.add(id);
    }
    setSelectedIds(newSelected);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSearch();
    }
  };

  // Stats
  const totalRecords = records.length;
  const inMaintenance = records.filter(r => r.manutencao).length;
  const normal = totalRecords - inMaintenance;

  if (loadingPermission) {
    return (
      <div className={styles.container}>
        <LoadingPlaceholder text="Carregando..." />
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h2>Gerenciador de Manutenção</h2>
      </div>

      {permission && !permission.hasPermission && (
        <Alert severity="warning" title="Acesso Limitado" className={styles.alert}>
          {permission.message || 'Você pode visualizar os registros, mas não pode fazer alterações.'}
        </Alert>
      )}

      {message && (
        <Alert
          severity={message.type}
          title={message.type === 'success' ? 'Sucesso' : message.type === 'error' ? 'Erro' : 'Aviso'}
          className={styles.alert}
          onRemove={() => setMessage(null)}
        >
          {message.text}
        </Alert>
      )}

      {/* Search Section */}
      <div className={styles.searchSection}>
        <div className={styles.searchRow}>
          <div className={styles.searchInput}>
            <Field label="Buscar por">
              <RadioButtonGroup
                options={[
                  { label: 'ID', value: 'id' },
                  { label: 'Nome', value: 'name' },
                ]}
                value={searchMode}
                onChange={(v) => setSearchMode(v as SearchMode)}
              />
            </Field>
            <TextArea
              value={searchText}
              onChange={(e) => setSearchText(e.currentTarget.value)}
              onKeyPress={handleKeyPress}
              placeholder={searchMode === 'id'
                ? 'Digite um ou mais IDs (separados por vírgula, espaço ou nova linha)...'
                : 'Digite um ou mais nomes para buscar...'}
              rows={3}
            />
            <p className={styles.searchHelp}>
              {searchMode === 'id'
                ? 'Exemplo: 123, 456, 789 ou um ID por linha'
                : 'Use vírgula ou nova linha para buscar múltiplos nomes'}
            </p>
          </div>

          <div className={styles.searchButtons}>
            <Button onClick={handleSearch} disabled={searching} icon={searching ? 'fa fa-spinner' : 'search'}>
              {searching ? 'Buscando...' : 'Buscar'}
            </Button>
          </div>
        </div>
      </div>

      {/* Results Section */}
      {records.length > 0 && (
        <div className={styles.resultsSection}>
          {/* Stats Bar */}
          <div className={styles.statsBar}>
            <span className={styles.statItem}>
              <Icon name="list-ul" /> Total: <strong>{totalRecords}</strong>
            </span>
            <span className={styles.statItem}>
              <Icon name="check-circle" style={{ color: '#73BF69' }} /> Normal: <strong>{normal}</strong>
            </span>
            <span className={styles.statItem}>
              <Icon name="exclamation-triangle" style={{ color: '#F2495C' }} /> Em Manutenção: <strong>{inMaintenance}</strong>
            </span>
          </div>

          {/* Bulk Actions */}
          {permission?.hasPermission && selectedIds.size > 0 && (
            <div className={styles.bulkActions}>
              <span>{selectedIds.size} selecionado(s)</span>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => handleBulkToggle(true)}
                disabled={loading}
              >
                Colocar em Manutenção
              </Button>
              <Button
                size="sm"
                variant="success"
                onClick={() => handleBulkToggle(false)}
                disabled={loading}
              >
                Remover Manutenção
              </Button>
            </div>
          )}

          {/* Results Table */}
          <table className={styles.table}>
            <thead className={styles.tableHeader}>
              <tr>
                {permission?.hasPermission && (
                  <th style={{ width: 40 }}>
                    <input
                      type="checkbox"
                      checked={selectedIds.size === records.length && records.length > 0}
                      onChange={toggleSelectAll}
                    />
                  </th>
                )}
                <th>ID</th>
                {tableConfig?.displayNameColumn && <th>Nome</th>}
                {tableConfig?.searchColumn && <th>{tableConfig.searchColumn}</th>}
                <th>Status</th>
                {tableConfig?.additionalColumns?.map(col => (
                  <th key={col}>{col}</th>
                ))}
                <th style={{ width: 150 }}>Ação</th>
              </tr>
            </thead>
            <tbody>
              {records.map((record) => (
                <tr key={String(record.id)} className={styles.tableRow}>
                  {permission?.hasPermission && (
                    <td>
                      <input
                        type="checkbox"
                        checked={selectedIds.has(record.id)}
                        onChange={() => toggleSelect(record.id)}
                      />
                    </td>
                  )}
                  <td>{String(record.id)}</td>
                  {tableConfig?.displayNameColumn && <td>{record.displayName || '-'}</td>}
                  {tableConfig?.searchColumn && <td>{String(record.searchValue ?? '-')}</td>}
                  <td>
                    <div className={styles.statusCell}>
                      <Badge
                        text={record.manutencao ? 'Em Manutenção' : 'Normal'}
                        color={record.manutencao ? 'red' : 'green'}
                        icon={record.manutencao ? 'exclamation-triangle' : 'check'}
                      />
                    </div>
                  </td>
                  {tableConfig?.additionalColumns?.map(col => (
                    <td key={col}>{String(record.additionalData?.[col] ?? '-')}</td>
                  ))}
                  <td className={styles.actionCell}>
                    <Button
                      size="sm"
                      variant={record.manutencao ? 'success' : 'destructive'}
                      onClick={() => handleToggleMaintenance(record)}
                      disabled={!permission?.hasPermission || loading}
                      title={!permission?.hasPermission ? 'Sem permissão' : ''}
                    >
                      {record.manutencao ? 'Normalizar' : 'Manutenção'}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {records.length === 0 && !searching && (
        <div className={styles.noResults}>
          <Icon name="search" size="xxl" />
          <p>Realize uma busca para visualizar os registros.</p>
        </div>
      )}

      {/* Confirm Modal */}
      <ConfirmModal
        isOpen={confirmModal.isOpen}
        title="Confirmar Alteração"
        body={
          confirmModal.isBulk ? (
            <p>
              Deseja {confirmModal.newStatus ? 'colocar em manutenção' : 'remover da manutenção'}{' '}
              <strong>{selectedIds.size}</strong> registro(s)?
            </p>
          ) : confirmModal.record ? (
            <p>
              Deseja {confirmModal.newStatus ? 'colocar em manutenção' : 'remover da manutenção'} o registro{' '}
              <strong>{confirmModal.record.displayName || confirmModal.record.id}</strong>?
            </p>
          ) : null
        }
        confirmText={confirmModal.newStatus ? 'Colocar em Manutenção' : 'Normalizar'}
        dismissText="Cancelar"
        onConfirm={confirmToggle}
        onDismiss={() => setConfirmModal({ isOpen: false, record: null, newStatus: false, isBulk: false })}
      />
    </div>
  );
};
