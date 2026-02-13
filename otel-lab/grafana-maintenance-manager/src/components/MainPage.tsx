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
  Modal,
  Tooltip
} from '@grafana/ui';
import { css } from '@emotion/css';
import { ServiceRecord, PermissionResponse, SearchResponse, UpdateResponse, ConfigResponse, AuditEntry, AuditResponse } from '../types';

interface Props extends AppRootProps {}

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    padding: ${theme.spacing(3)};
    max-width: 1400px;
  `,
  header: css`
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: ${theme.spacing(2)};
  `,
  headerActions: css`
    display: flex;
    gap: ${theme.spacing(1)};
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
    display: flex;
    gap: ${theme.spacing(0.5)};
    justify-content: flex-end;
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
    margin-bottom: ${theme.spacing(2)};
  `,
  auditModal: css`
    min-width: 800px;
  `,
  auditTable: css`
    width: 100%;
    border-collapse: collapse;
    font-size: ${theme.typography.bodySmall.fontSize};
    th, td {
      padding: ${theme.spacing(1)};
      border-bottom: 1px solid ${theme.colors.border.weak};
      text-align: left;
    }
    th {
      background: ${theme.colors.background.secondary};
      font-weight: ${theme.typography.fontWeightMedium};
    }
  `,
  auditActions: css`
    display: flex;
    gap: ${theme.spacing(1)};
    margin-bottom: ${theme.spacing(2)};
  `,
  iconButton: css`
    background: transparent;
    border: none;
    cursor: pointer;
    padding: ${theme.spacing(0.5)};
    border-radius: ${theme.shape.radius.default};
    &:hover {
      background: ${theme.colors.background.secondary};
    }
  `,
});

type SearchMode = 'id' | 'name';

export const MainPage: React.FC<Props> = () => {
  const styles = useStyles2(getStyles);

  // Search state
  const [searchText, setSearchText] = useState('');
  const [searchMode, setSearchMode] = useState<SearchMode>('id');
  const [records, setRecords] = useState<ServiceRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [searching, setSearching] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error' | 'warning'; text: string } | null>(null);

  // Config state
  const [config, setConfig] = useState<ConfigResponse | null>(null);

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

  // Audit modal state
  const [auditModal, setAuditModal] = useState<{
    isOpen: boolean;
    recordId: string | null;
    recordName: string | null;
  }>({ isOpen: false, recordId: null, recordName: null });
  const [auditEntries, setAuditEntries] = useState<AuditEntry[]>([]);
  const [loadingAudit, setLoadingAudit] = useState(false);

  useEffect(() => {
    checkPermission();
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const response: ConfigResponse = await getBackendSrv().get('/api/plugins/grafana-maintenance-manager/resources/config');
      setConfig(response);
    } catch (error) {
      console.error('Failed to load config:', error);
    }
  };

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
        userEmail: '',
        message: 'Erro ao verificar permissões'
      });
    } finally {
      setLoadingPermission(false);
    }
  };

  const parseSearchValues = (text: string, byName: boolean): string[] => {
    // For name search: split only by comma, semicolon, or newline (preserve spaces in names)
    // For ID search: split by comma, semicolon, newline, or whitespace
    const separator = byName ? /[,;\n]+/ : /[,;\n\s]+/;
    return text
      .split(separator)
      .map(v => v.trim())
      .filter(v => v.length > 0);
  };

  const handleSearch = async () => {
    const values = parseSearchValues(searchText, searchMode === 'name');
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
        const idsToUpdate = Array.from(selectedIds);
        let successCount = 0;
        let errorCount = 0;

        for (const id of idsToUpdate) {
          try {
            const record = records.find(r => r.id === id);
            const response: UpdateResponse = await getBackendSrv().post(
              '/api/plugins/grafana-maintenance-manager/resources/update',
              { id, manutencao: confirmModal.newStatus, recordName: record?.nome || '' }
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
        const response: UpdateResponse = await getBackendSrv().post(
          '/api/plugins/grafana-maintenance-manager/resources/update',
          {
            id: confirmModal.record.id,
            manutencao: confirmModal.newStatus,
            recordName: confirmModal.record.nome || ''
          }
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

  // Audit functions
  const openAuditModal = async (recordId?: string, recordName?: string) => {
    setAuditModal({
      isOpen: true,
      recordId: recordId || null,
      recordName: recordName || null
    });
    await loadAuditEntries(recordId);
  };

  const loadAuditEntries = async (recordId?: string) => {
    setLoadingAudit(true);
    try {
      const params = new URLSearchParams();
      if (recordId) {
        params.append('recordId', recordId);
      }
      params.append('limit', '100');

      const response: AuditResponse = await getBackendSrv().get(
        `/api/plugins/grafana-maintenance-manager/resources/audit?${params.toString()}`
      );

      if (response.success) {
        setAuditEntries(response.entries || []);
      } else {
        setAuditEntries([]);
        console.error('Audit error:', response.error);
      }
    } catch (error) {
      console.error('Failed to load audit:', error);
      setAuditEntries([]);
    } finally {
      setLoadingAudit(false);
    }
  };

  const exportAuditCsv = () => {
    const params = new URLSearchParams();
    if (auditModal.recordId) {
      params.append('recordId', auditModal.recordId);
    }
    window.open(`/api/plugins/grafana-maintenance-manager/resources/audit/export?${params.toString()}`, '_blank');
  };

  const closeAuditModal = () => {
    setAuditModal({ isOpen: false, recordId: null, recordName: null });
    setAuditEntries([]);
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
        <div className={styles.headerActions}>
          {config?.hasAuditTable && (
            <Button
              variant="secondary"
              icon="history"
              onClick={() => openAuditModal()}
            >
              Auditoria Geral
            </Button>
          )}
        </div>
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
                ? 'Exemplo: 123, 456, 789 ou um ID por linha (separados por espaco, virgula ou nova linha)'
                : 'Digite o nome completo. Use virgula ou nova linha para buscar multiplos nomes.'}
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
                <th>Nome</th>
                <th>Status</th>
                <th style={{ width: 150 }}>Acoes</th>
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
                  <td>{record.nome || '-'}</td>
                  <td>
                    <div className={styles.statusCell}>
                      <Badge
                        text={record.manutencao ? 'Em Manutencao' : 'Normal'}
                        color={record.manutencao ? 'red' : 'green'}
                        icon={record.manutencao ? 'exclamation-triangle' : 'check'}
                      />
                    </div>
                  </td>
                  <td>
                    <div className={styles.actionCell}>
                      <Button
                        size="sm"
                        variant={record.manutencao ? 'success' : 'destructive'}
                        onClick={() => handleToggleMaintenance(record)}
                        disabled={!permission?.hasPermission || loading}
                        title={!permission?.hasPermission ? 'Sem permissao' : ''}
                      >
                        {record.manutencao ? 'Normalizar' : 'Manutencao'}
                      </Button>
                      {config?.hasAuditTable && (
                        <Tooltip content="Ver historico">
                          <button
                            className={styles.iconButton}
                            onClick={() => openAuditModal(String(record.id), record.nome)}
                          >
                            <Icon name="history" />
                          </button>
                        </Tooltip>
                      )}
                    </div>
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
              <strong>{confirmModal.record.nome || confirmModal.record.id}</strong>?
            </p>
          ) : null
        }
        confirmText={confirmModal.newStatus ? 'Colocar em Manutenção' : 'Normalizar'}
        dismissText="Cancelar"
        onConfirm={confirmToggle}
        onDismiss={() => setConfirmModal({ isOpen: false, record: null, newStatus: false, isBulk: false })}
      />

      {/* Audit Modal */}
      <Modal
        title={auditModal.recordId
          ? `Histórico de Alterações - ${auditModal.recordName || auditModal.recordId}`
          : 'Histórico Geral de Alterações'}
        isOpen={auditModal.isOpen}
        onDismiss={closeAuditModal}
        className={styles.auditModal}
      >
        <div className={styles.auditActions}>
          <Button
            variant="secondary"
            icon="download-alt"
            onClick={exportAuditCsv}
            disabled={loadingAudit || auditEntries.length === 0}
          >
            Exportar CSV
          </Button>
          <Button
            variant="secondary"
            icon="sync"
            onClick={() => loadAuditEntries(auditModal.recordId || undefined)}
            disabled={loadingAudit}
          >
            Atualizar
          </Button>
        </div>

        {loadingAudit ? (
          <LoadingPlaceholder text="Carregando histórico..." />
        ) : auditEntries.length === 0 ? (
          <p>Nenhum registro de auditoria encontrado.</p>
        ) : (
          <table className={styles.auditTable}>
            <thead>
              <tr>
                <th>Data/Hora</th>
                <th>Usuário</th>
                <th>Email</th>
                <th>Ação</th>
                {!auditModal.recordId && <th>Registro</th>}
                <th>De</th>
                <th>Para</th>
              </tr>
            </thead>
            <tbody>
              {auditEntries.map((entry) => (
                <tr key={entry.id}>
                  <td>{entry.timestampStr}</td>
                  <td>{entry.userLogin}</td>
                  <td>{entry.userEmail}</td>
                  <td>{entry.action}</td>
                  {!auditModal.recordId && <td>{entry.recordName || entry.recordId}</td>}
                  <td>
                    <Badge
                      text={entry.oldValue ? 'Manutenção' : 'Normal'}
                      color={entry.oldValue ? 'red' : 'green'}
                    />
                  </td>
                  <td>
                    <Badge
                      text={entry.newValue ? 'Manutenção' : 'Normal'}
                      color={entry.newValue ? 'red' : 'green'}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Modal>
    </div>
  );
};
