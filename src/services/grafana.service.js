import fetch from 'node-fetch';

export class GrafanaService {
  constructor(url, token) {
    this.url = url;
    this.token = token;
  }

  async getOrCreateFolder(folderName) {
    try {
      const response = await fetch(`${this.url}/api/folders`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${this.token}`,
          'Content-Type': 'application/json'
        }
      });
            
      if (!response.ok) {
        await response.text();
        throw new Error(`Failed to fetch folders: ${response.statusText}`);
      }
      
      const folders = await response.json();
      const existing = folders.find(f => f.title === folderName);
      
      if (existing) return existing.id;

      const createResp = await fetch(`${this.url}/api/folders`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${this.token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ 
          title: folderName,
          uid: folderName.toLowerCase().replace(/[^a-z0-9]/g, '-')
        })
      });

      if (!createResp.ok) {
        const errorText = await createResp.text();
        throw new Error(`Failed to create folder: ${createResp.statusText} - ${errorText}`);
      }

      const newFolder = await createResp.json();
      return newFolder.id;
    } catch (error) {
      throw new Error(`Grafana folder operation failed: ${error.message}`);
    }
  }

  async createDashboard(dashboardData) {
    try {
      const { title, datasource, metrics, folder } = dashboardData;
      
      let folderId = 0; // Default: General (root level)
      
      if (folder && folder.toLowerCase() !== 'general') {
        // User wants a specific folder - try to get or create it
        folderId = await this.getOrCreateFolder(folder);
      }

      // Create panels for each metric
      const panels = metrics.map((metric, index) => ({
        id: index + 1,
        type: 'timeseries',
        title: `${metric}`,
        datasource: { type: 'prometheus', uid: datasource },
        targets: [{
          expr: metric,
          refId: String.fromCharCode(65 + index),
          datasource: { type: 'prometheus', uid: datasource }
        }],
        gridPos: {
          x: (index % 2) * 12,
          y: Math.floor(index / 2) * 8,
          w: 12,
          h: 8
        },
        fieldConfig: {
          defaults: {
            unit: 'short',
            custom: {
              drawStyle: 'line',
              lineInterpolation: 'linear',
              barAlignment: 0,
              lineWidth: 1,
              fillOpacity: 0.1,
              gradientMode: 'none'
            }
          }
        }
      }));

      const dashboard = {
        dashboard: {
          id: null,
          title,
          tags: ['ai-generated'],
          timezone: 'browser',
          panels,
          time: {
            from: 'now-1h',
            to: 'now'
          },
          timepicker: {},
          templating: { list: [] },
          annotations: { list: [] },
          refresh: '5s',
          schemaVersion: 27,
          version: 0,
          links: []
        },
        folderId,
        overwrite: true
      };

      const response = await fetch(`${this.url}/api/dashboards/db`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${this.token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(dashboard)
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Failed to create dashboard: ${response.statusText} - ${errorText}`);
      }

      return await response.json();
    } catch (error) {
      throw new Error(`Dashboard creation failed: ${error.message}`);
    }
  }
}