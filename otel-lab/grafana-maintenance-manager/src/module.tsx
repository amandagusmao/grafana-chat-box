import React, { Suspense } from 'react';
import { AppPlugin, type AppRootProps } from '@grafana/data';
import { LoadingPlaceholder } from '@grafana/ui';
import { ConfigPage } from './components/ConfigPage';
import { MainPage } from './components/MainPage';

const App = (props: AppRootProps) => {
  const { path } = props;

  // Renderiza a página de configuração
  if (path === '/a/grafana-maintenance-manager/config') {
    return (
      <Suspense fallback={<LoadingPlaceholder text="Carregando configurações..." />}>
        <ConfigPage {...props} />
      </Suspense>
    );
  }

  // Renderiza a página principal (gerenciamento)
  return (
    <Suspense fallback={<LoadingPlaceholder text="Carregando Maintenance Manager..." />}>
      <MainPage {...props} />
    </Suspense>
  );
};

export const plugin = new AppPlugin<{}>().setRootPage(App);
