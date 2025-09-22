import React, { Suspense } from 'react';
import { AppPlugin, type AppRootProps } from '@grafana/data';
import { LoadingPlaceholder } from '@grafana/ui';
import { DashboardExportPage } from './components/DashboardExportPage';

const App = (props: AppRootProps) => {
  const { path } = props;

  // Renderiza o DashboardExportPage quando acessar a rota do plugin
  if (path === '/a/grafana-dashboard-exporter/export') {
    return (
      <Suspense fallback={<LoadingPlaceholder text="Carregando Dashboard Exporter..." />}>
        <DashboardExportPage />
      </Suspense>
    );
  }

  // Página padrão/root
  return (
    <Suspense fallback={<LoadingPlaceholder text="Carregando Dashboard Exporter..." />}>
      <DashboardExportPage />
    </Suspense>
  );
};

export const plugin = new AppPlugin<{}>().setRootPage(App);