// Plugin Settings
export interface PluginSettings {
  jsonData: {
    // Connection
    datasourceUid?: string;
    grafanaUrl?: string;
    // Query configuration
    selectQuery?: string;
    updateTable?: string;
    idColumn?: string;          // Column for search and UPDATE (e.g., "id_cadastro")
    nameColumn?: string;        // Column that displays the name (e.g., "nome")
    maintenanceColumn?: string; // Column to update (0/1)
    // Audit
    auditTable?: string;
    // Access control
    allowedOrgId?: string;
  };
  secureJsonFields: {
    grafanaToken?: boolean;
  };
}

// Datasource Info
export interface DatasourceInfo {
  uid: string;
  name: string;
  type: string;
  isDefault: boolean;
}

// Service Record from API (dynamic structure)
export interface ServiceRecord {
  id: any;
  nome: string;
  manutencao: boolean;
  fields?: Record<string, any>;
}

// Search Request
export interface SearchRequest {
  searchValues: string[];
  searchByName: boolean;
}

// Search Response
export interface SearchResponse {
  success: boolean;
  records: ServiceRecord[];
  columns?: string[];
  error?: string;
}

// Update Request
export interface UpdateRequest {
  id: any;
  manutencao: boolean;
  recordName?: string;
}

// Update Response
export interface UpdateResponse {
  success: boolean;
  message?: string;
  error?: string;
}

// Permission Check Response
export interface PermissionResponse {
  hasPermission: boolean;
  currentOrgId: number;
  allowedOrgId: number;
  userLogin: string;
  userEmail: string;
  message?: string;
}

// Config Response
export interface ConfigResponse {
  success: boolean;
  idColumn?: string;
  nameColumn?: string;
  maintenanceColumn?: string;
  hasAuditTable?: boolean;
  error?: string;
}

// Audit Entry
export interface AuditEntry {
  id: number;
  userLogin: string;
  userEmail: string;
  action: string;
  recordId: string;
  recordName: string;
  oldValue: boolean;
  newValue: boolean;
  timestamp: string;
  timestampStr: string;
}

// Audit Request
export interface AuditRequest {
  recordId?: string;
  limit?: number;
  offset?: number;
}

// Audit Response
export interface AuditResponse {
  success: boolean;
  entries: AuditEntry[];
  total: number;
  error?: string;
}
