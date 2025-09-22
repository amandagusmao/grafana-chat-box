import React, { useState, useEffect } from 'react';
import {
  Button,
  Field,
  Input,
  Select,
  Alert,
  Spinner,
  useTheme2,
  Card,
  Checkbox,
  VerticalGroup,
  HorizontalGroup,
  Modal,
  TextArea,
} from '@grafana/ui';
import { SelectableValue, AppPluginMeta, PluginConfigPageProps, GrafanaTheme2 } from '@grafana/data';
import { getBackendSrv, locationService } from '@grafana/runtime';
import { css } from '@emotion/css';

interface Dashboard {
  id: number;
  uid: string;
  title: string;
  tags: string[];
  folderTitle?: string;
  uri: string;
  type: string;
  isStarred: boolean;
}

interface GitHubConfig {
  token: string;
  owner: string;
  repo: string;
  branch: string;
  path: string;
}

export const DashboardExportPage: React.FC = () => {
  const theme = useTheme2();
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [selectedDashboards, setSelectedDashboards] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [tagFilter, setTagFilter] = useState<string>('');
  const [folderFilter, setFolderFilter] = useState<string>('');
  const [showConfig, setShowConfig] = useState(false);
  const [config, setConfig] = useState<GitHubConfig>({
    token: '',
    owner: '',
    repo: '',
    branch: 'main',
    path: 'dashboards'
  });
  const [alert, setAlert] = useState<{ type: 'success' | 'error' | 'info'; message: string } | null>(null);
  const [allTags, setAllTags] = useState<string[]>([]);
  const [allFolders, setAllFolders] = useState<string[]>([]);
  const [isAdmin, setIsAdmin] = useState<boolean>(false);
  const [userRole, setUserRole] = useState<string>('');

  const styles = getStyles(theme);

  useEffect(() => {
    loadDashboards();
    loadConfigFromStorage();
    checkUserRole();
  }, []);

  const checkUserRole = async () => {
    try {
      // Get current user from Grafana API
      const userResponse = await getBackendSrv().get('/api/user');

      if (userResponse) {
        setUserRole(userResponse.orgRole || '');
        // Check if user is admin (orgRole = 'Admin' or isGrafanaAdmin = true)
        const adminStatus = userResponse.orgRole === 'Admin' || userResponse.isGrafanaAdmin;
        setIsAdmin(adminStatus);

        // If not admin, show info message
        if (!adminStatus) {
          setAlert({
            type: 'info',
            message: 'Apenas administradores podem configurar as credenciais do GitHub'
          });
        }
      }
    } catch (error) {
      console.error('Error checking user role:', error);
      setIsAdmin(false);
      setAlert({
        type: 'error',
        message: 'Erro ao verificar permissões do usuário'
      });
    }
  };

  const loadConfigFromStorage = () => {
    const savedConfig = localStorage.getItem('grafana-github-export-config');
    if (savedConfig) {
      setConfig(JSON.parse(savedConfig));
    }
  };

  const saveConfigToStorage = (newConfig: GitHubConfig) => {
    localStorage.setItem('grafana-github-export-config', JSON.stringify(newConfig));
  };

  const loadDashboards = async () => {
    setLoading(true);
    try {
      const response = await getBackendSrv().get('/api/search', {
        type: 'dash-db',
        limit: 1000
      });

      setDashboards(response);

      // Extract unique tags and folders
      const tags = new Set<string>();
      const folders = new Set<string>();

      response.forEach((dashboard: Dashboard) => {
        dashboard.tags?.forEach(tag => tags.add(tag));
        if (dashboard.folderTitle) {
          folders.add(dashboard.folderTitle);
        }
      });

      setAllTags(Array.from(tags).sort());
      setAllFolders(Array.from(folders).sort());

    } catch (error) {
      console.error('Error loading dashboards:', error);
      setAlert({ type: 'error', message: 'Erro ao carregar dashboards' });
    } finally {
      setLoading(false);
    }
  };

  const exportToGitHub = async () => {
    if (selectedDashboards.size === 0) {
      setAlert({ type: 'error', message: 'Selecione pelo menos um dashboard' });
      return;
    }

    if (!config.token || !config.owner || !config.repo) {
      setAlert({
        type: 'error',
        message: !isAdmin
          ? 'GitHub não configurado. Entre em contato com um administrador.'
          : 'Configure as credenciais do GitHub'
      });
      return;
    }

    setExporting(true);
    let successCount = 0;
    let errorCount = 0;

    try {
      for (const dashboardUid of selectedDashboards) {
        try {
          // Get dashboard JSON
          const dashboardResponse = await getBackendSrv().get(`/api/dashboards/uid/${dashboardUid}`);
          const dashboard = dashboardResponse.dashboard;

          // Prepare file content
          const fileName = `${dashboard.title.replace(/[^a-zA-Z0-9\s-]/g, '').replace(/\s+/g, '-').toLowerCase()}.json`;
          const filePath = config.path ? `${config.path}/${fileName}` : fileName;
          const fileContent = JSON.stringify(dashboard, null, 2);

          // Upload to GitHub
          await uploadToGitHub(filePath, fileContent, `Add dashboard: ${dashboard.title}`);
          successCount++;

        } catch (error) {
          console.error(`Error exporting dashboard ${dashboardUid}:`, error);
          errorCount++;
        }
      }

      if (successCount > 0) {
        setAlert({
          type: 'success',
          message: `${successCount} dashboard(s) exportado(s) com sucesso${errorCount > 0 ? `. ${errorCount} erro(s).` : ''}`
        });
        setSelectedDashboards(new Set());
      } else {
        setAlert({ type: 'error', message: 'Falha ao exportar dashboards' });
      }

    } catch (error) {
      console.error('Export error:', error);
      setAlert({ type: 'error', message: 'Erro durante a exportação' });
    } finally {
      setExporting(false);
    }
  };

  const uploadToGitHub = async (path: string, content: string, message: string) => {
    const url = `https://api.github.com/repos/${config.owner}/${config.repo}/contents/${path}`;

    // Check if file exists to get sha
    let sha: string | undefined;
    try {
      const existingFileResponse = await fetch(url, {
        headers: {
          'Authorization': `token ${config.token}`,
          'Accept': 'application/vnd.github.v3+json'
        }
      });

      if (existingFileResponse.ok) {
        const existingFile = await existingFileResponse.json();
        sha = existingFile.sha;
      }
    } catch (error) {
      // File doesn't exist, which is fine
    }

    const payload: any = {
      message,
      content: btoa(unescape(encodeURIComponent(content))),
      branch: config.branch
    };

    if (sha) {
      payload.sha = sha;
    }

    const response = await fetch(url, {
      method: 'PUT',
      headers: {
        'Authorization': `token ${config.token}`,
        'Accept': 'application/vnd.github.v3+json',
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(payload)
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(`GitHub API error: ${error.message}`);
    }

    return response.json();
  };

  const filteredDashboards = dashboards.filter(dashboard => {
    const matchesSearch = dashboard.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         dashboard.tags?.some(tag => tag.toLowerCase().includes(searchTerm.toLowerCase()));

    const matchesTag = !tagFilter || dashboard.tags?.includes(tagFilter);
    const matchesFolder = !folderFilter || dashboard.folderTitle === folderFilter;

    return matchesSearch && matchesTag && matchesFolder;
  });

  const handleSelectAll = () => {
    if (selectedDashboards.size === filteredDashboards.length) {
      setSelectedDashboards(new Set());
    } else {
      setSelectedDashboards(new Set(filteredDashboards.map(d => d.uid)));
    }
  };

  const handleDashboardSelect = (uid: string) => {
    const newSelected = new Set(selectedDashboards);
    if (newSelected.has(uid)) {
      newSelected.delete(uid);
    } else {
      newSelected.add(uid);
    }
    setSelectedDashboards(newSelected);
  };

  const handleConfigSave = () => {
    if (!isAdmin) {
      setAlert({ type: 'error', message: 'Apenas administradores podem salvar configurações do GitHub' });
      return;
    }

    saveConfigToStorage(config);
    setShowConfig(false);
    setAlert({ type: 'success', message: 'Configuração salva com sucesso' });
  };

  const handleConfigOpen = () => {
    if (!isAdmin) {
      setAlert({ type: 'error', message: 'Apenas administradores podem configurar as credenciais do GitHub' });
      return;
    }
    setShowConfig(true);
  };

  const tagOptions: SelectableValue[] = [
    { label: 'Todas as tags', value: '' },
    ...allTags.map(tag => ({ label: tag, value: tag }))
  ];

  const folderOptions: SelectableValue[] = [
    { label: 'Todas as pastas', value: '' },
    ...allFolders.map(folder => ({ label: folder, value: folder }))
  ];

  return (
    <div className={styles.container}>
      <VerticalGroup spacing="lg">
        <div className={styles.header}>
          <div className={styles.headerTitle}>
            <h2>Export Dashboards para GitHub</h2>
            {!isAdmin && (
              <div className={styles.roleInfo}>
                <span className={styles.roleLabel}>Perfil: {userRole}</span>
                <span className={styles.roleNote}>
                  (Apenas administradores podem configurar GitHub)
                </span>
              </div>
            )}
          </div>
          <HorizontalGroup>
            {isAdmin && (
              <Button
                variant="secondary"
                icon="cog"
                onClick={handleConfigOpen}
              >
                Configurar GitHub
              </Button>
            )}
            <Button
              variant="primary"
              icon="cloud-upload"
              onClick={exportToGitHub}
              disabled={selectedDashboards.size === 0 || exporting || (!config.token || !config.owner || !config.repo)}
            >
              {exporting ? <Spinner size="sm" /> : null}
              Exportar Selecionados ({selectedDashboards.size})
            </Button>
            {!isAdmin && (
              <Button
                variant="secondary"
                icon="info-circle"
                onClick={() => setAlert({
                  type: 'info',
                  message: `Configuração GitHub: ${config.owner ? `${config.owner}/${config.repo}` : 'Não configurado'}`
                })}
              >
                Ver Configuração
              </Button>
            )}
          </HorizontalGroup>
        </div>

        {alert && (
          <Alert
            title={alert.message}
            severity={alert.type}
            onRemove={() => setAlert(null)}
          />
        )}

        <div className={styles.filters}>
          <HorizontalGroup spacing="md">
            <Field label="Buscar">
              <Input
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.currentTarget.value)}
                placeholder="Nome do dashboard ou tag..."
                width={30}
              />
            </Field>
            <Field label="Filtrar por tag">
              <Select
                value={tagFilter}
                onChange={(value) => setTagFilter(value?.value || '')}
                options={tagOptions}
                width={25}
              />
            </Field>
            <Field label="Filtrar por pasta">
              <Select
                value={folderFilter}
                onChange={(value) => setFolderFilter(value?.value || '')}
                options={folderOptions}
                width={25}
              />
            </Field>
          </HorizontalGroup>
        </div>

        <div className={styles.dashboardSection}>
          <div className={styles.dashboardHeader}>
            <h3 className={styles.sectionTitle}>📋 Dashboards Disponíveis</h3>
          </div>

          <div className={styles.dashboardControls}>
            <HorizontalGroup spacing="md">
              <Button
                variant="secondary"
                size="sm"
                onClick={handleSelectAll}
              >
                {selectedDashboards.size === filteredDashboards.length ? 'Desmarcar Todos' : 'Selecionar Todos'}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                icon="sync"
                onClick={loadDashboards}
                disabled={loading}
              >
                Atualizar
              </Button>
            </HorizontalGroup>
          </div>

          <Card className={styles.dashboardCard}>
            {loading ? (
              <div className={styles.loading}>
                <Spinner size="lg" />
                <span>Carregando dashboards...</span>
              </div>
            ) : (
              <div className={styles.dashboardList}>
                {filteredDashboards.map((dashboard) => (
                  <div key={dashboard.uid} className={styles.dashboardItem}>
                    <Checkbox
                      value={selectedDashboards.has(dashboard.uid)}
                      onChange={() => handleDashboardSelect(dashboard.uid)}
                    />
                    <div className={styles.dashboardInfo}>
                      <div className={styles.dashboardTitle}>{dashboard.title}</div>
                      <div className={styles.dashboardMeta}>
                        {dashboard.folderTitle && (
                          <span className={styles.folder}>📁 {dashboard.folderTitle}</span>
                        )}
                        {dashboard.tags && dashboard.tags.length > 0 && (
                          <div className={styles.tags}>
                            {dashboard.tags.map(tag => (
                              <span key={tag} className={styles.tag}>{tag}</span>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                ))}

                {filteredDashboards.length === 0 && !loading && (
                  <div className={styles.emptyState}>
                    Nenhum dashboard encontrado
                  </div>
                )}
              </div>
            )}
          </Card>
        </div>

        {showConfig && (
          <Modal
            title="Configurar GitHub"
            isOpen={showConfig}
            onDismiss={() => setShowConfig(false)}
          >
            <VerticalGroup spacing="md">
              <Field label="GitHub Token" description="Token pessoal com permissões de repositório">
                <Input
                  type="password"
                  value={config.token}
                  onChange={(e) => setConfig({...config, token: e.currentTarget.value})}
                  placeholder="ghp_xxxxxxxxxxxx"
                />
              </Field>

              <Field label="Owner/Organização">
                <Input
                  value={config.owner}
                  onChange={(e) => setConfig({...config, owner: e.currentTarget.value})}
                  placeholder="username ou organization"
                />
              </Field>

              <Field label="Repositório">
                <Input
                  value={config.repo}
                  onChange={(e) => setConfig({...config, repo: e.currentTarget.value})}
                  placeholder="nome-do-repositorio"
                />
              </Field>

              <Field label="Branch">
                <Input
                  value={config.branch}
                  onChange={(e) => setConfig({...config, branch: e.currentTarget.value})}
                  placeholder="main"
                />
              </Field>

              <Field label="Caminho no repositório" description="Pasta onde os dashboards serão salvos">
                <Input
                  value={config.path}
                  onChange={(e) => setConfig({...config, path: e.currentTarget.value})}
                  placeholder="dashboards"
                />
              </Field>

              <HorizontalGroup>
                <Button variant="primary" onClick={handleConfigSave}>
                  Salvar
                </Button>
                <Button variant="secondary" onClick={() => setShowConfig(false)}>
                  Cancelar
                </Button>
              </HorizontalGroup>
            </VerticalGroup>
          </Modal>
        )}
      </VerticalGroup>
    </div>
  );
};

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    padding: ${theme.spacing(3)};
    max-width: 1200px;
  `,
  header: css`
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: ${theme.spacing(2)};
  `,
  headerTitle: css`
    display: flex;
    flex-direction: column;
    gap: ${theme.spacing(1)};
  `,
  roleInfo: css`
    display: flex;
    flex-direction: column;
    gap: ${theme.spacing(0.5)};
  `,
  roleLabel: css`
    font-size: ${theme.typography.bodySmall.fontSize};
    color: ${theme.colors.text.secondary};
    font-weight: ${theme.typography.fontWeightMedium};
  `,
  roleNote: css`
    font-size: ${theme.typography.bodySmall.fontSize};
    color: ${theme.colors.warning.text};
    font-style: italic;
  `,
  filters: css`
    padding: ${theme.spacing(2)};
    background: ${theme.colors.background.secondary};
    border-radius: ${theme.shape.borderRadius()};
  `,
  dashboardSection: css`
    display: flex;
    flex-direction: column;
    gap: ${theme.spacing(2)};
  `,
  dashboardHeader: css`
    display: flex;
    align-items: center;
    margin-bottom: ${theme.spacing(1)};
  `,
  sectionTitle: css`
    margin: 0;
    font-size: ${theme.typography.h4.fontSize};
    font-weight: ${theme.typography.fontWeightMedium};
    color: ${theme.colors.text.primary};
  `,
  dashboardControls: css`
    display: flex;
    justify-content: flex-start;
    align-items: center;
    margin-bottom: ${theme.spacing(2)};
  `,
  dashboardCard: css`
    overflow: hidden;
  `,
  loading: css`
    display: flex;
    align-items: center;
    justify-content: center;
    padding: ${theme.spacing(4)};
    gap: ${theme.spacing(2)};
  `,
  dashboardList: css`
    max-height: 600px;
    overflow-y: auto;
  `,
  dashboardItem: css`
    display: flex;
    align-items: flex-start;
    padding: ${theme.spacing(2)};
    border-bottom: 1px solid ${theme.colors.border.weak};
    gap: ${theme.spacing(2)};

    &:hover {
      background: ${theme.colors.background.secondary};
    }
  `,
  dashboardInfo: css`
    flex: 1;
  `,
  dashboardTitle: css`
    font-weight: ${theme.typography.fontWeightMedium};
    margin-bottom: ${theme.spacing(1)};
  `,
  dashboardMeta: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(2)};
    font-size: ${theme.typography.bodySmall.fontSize};
    color: ${theme.colors.text.secondary};
  `,
  folder: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(0.5)};
  `,
  tags: css`
    display: flex;
    gap: ${theme.spacing(1)};
    flex-wrap: wrap;
  `,
  tag: css`
    background: ${theme.colors.background.canvas};
    padding: ${theme.spacing(0.5, 1)};
    border-radius: ${theme.shape.borderRadius()};
    font-size: ${theme.typography.bodySmall.fontSize};
    border: 1px solid ${theme.colors.border.weak};
  `,
  emptyState: css`
    text-align: center;
    padding: ${theme.spacing(4)};
    color: ${theme.colors.text.secondary};
  `
});