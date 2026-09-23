import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import '@fontsource/manrope/cyrillic-400.css';
import '@fontsource/manrope/cyrillic-600.css';
import '@fontsource/manrope/cyrillic-700.css';
import '@fontsource/manrope/cyrillic-800.css';
import '@fontsource/manrope/latin-400.css';
import '@fontsource/manrope/latin-700.css';
import './app/styles/reference.css';
import './app/styles/app.css';
import './app/styles/workflow.css';
import { App } from './app/App';
import { SessionProvider } from './features/session/SessionProvider';
import { DraftProvider } from './features/issues/drafts';
const client = new QueryClient({
  defaultOptions: { queries: { retry: false, staleTime: 15000 }, mutations: { retry: false } },
});
class Boundary extends React.Component<{ children: React.ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? (
      <div className="app">
        <main className="page-content">
          <h1>Не удалось открыть страницу</h1>
          <p>Обновите приложение и попробуйте снова.</p>
          <button onClick={() => window.location.reload()}>Обновить</button>
        </main>
      </div>
    ) : (
      this.props.children
    );
  }
}
ReactDOM.createRoot(document.getElementById('root')!).render(
  <Boundary>
    <QueryClientProvider client={client}>
      <BrowserRouter>
        <DraftProvider>
          <SessionProvider>
            <App />
          </SessionProvider>
        </DraftProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </Boundary>,
);
