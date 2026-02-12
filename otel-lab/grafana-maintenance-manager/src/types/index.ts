// Plugin Settings
export interface PluginSettings {
  jsonData: {
    // Connection
    datasourceUid?: string;
    grafanaUrl?: string;
    // Table configuration
    tableName?: string;
    primaryKeyColumn?: string;
    maintenanceColumn?: string;
    searchColumn?: string;
    displayNameColumn?: string;
    additionalColumns?: string;
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

// Table Config from API
export interface TableConfig {
  primaryKeyColumn: string;
  maintenanceColumn: string;
  searchColumn: string;
  displayNameColumn: string;
  additionalColumns: string[];
}

// Service Record from API (dynamic structure)
export interface ServiceRecord {
  id: any;
  displayName: string;
  searchValue: any;
  manutencao: boolean;
  additionalData?: Record<string, any>;
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
  config?: TableConfig;
  error?: string;
}

// Update Request
export interface UpdateRequest {
  id: any;
  manutencao: boolean;
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
  message?: string;
}

// Config Response
export interface ConfigResponse {
  success: boolean;
  tableConfig?: TableConfig;
  error?: string;
}
