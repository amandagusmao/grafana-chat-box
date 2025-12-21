import React, { Suspense } from 'react';
import { AppPlugin, type AppRootProps } from '@grafana/data';
import { LoadingPlaceholder } from '@grafana/ui';
import { ChatAppPage } from './components/ChatAppPage';
import { ConfigPage } from './components/ConfigPage';

const App = (props: AppRootProps) => {
  const { path } = props;

  // Renderiza a página de configuração
  if (path === '/a/grafana-chat-assistant/config') {
    return (
      <Suspense fallback={<LoadingPlaceholder text="Carregando configurações..." />}>
        <ConfigPage {...props} />
      </Suspense>
    );
  }

  // Renderiza o ChatAppPage quando acessar a rota do plugin ou rota padrão
  return (
    <Suspense fallback={<LoadingPlaceholder text="Carregando Chat Assistant..." />}>
      <ChatAppPage {...props} />
    </Suspense>
  );
};

export const plugin = new AppPlugin<{}>().setRootPage(App);