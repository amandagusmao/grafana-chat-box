import React, { Suspense } from 'react';
import { AppPlugin, type AppRootProps } from '@grafana/data';
import { LoadingPlaceholder } from '@grafana/ui';
import { ChatAppPage } from './components/ChatAppPage';

const App = (props: AppRootProps) => {
  const { path } = props;
  
  // Renderiza o ChatAppPage quando acessar a rota do plugin
  if (path === '/a/grafana-chat-assistant/chat') {
    return (
      <Suspense fallback={<LoadingPlaceholder text="Carregando Chat Assistant..." />}>
        <ChatAppPage {...props} />
      </Suspense>
    );
  }
  
  // Página padrão/root
  return (
    <Suspense fallback={<LoadingPlaceholder text="Carregando Chat Assistant..." />}>
      <ChatAppPage {...props} />
    </Suspense>
  );
};

export const plugin = new AppPlugin<{}>().setRootPage(App);