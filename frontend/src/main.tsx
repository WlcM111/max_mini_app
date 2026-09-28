import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './shared/styles/tokens.css';
import './shared/styles/base.css';
import './shared/styles/components.css';
import './shared/styles/pages.css';
import { App } from './app/App';
import { ErrorBoundary } from './app/ErrorBoundary';
import { flushTelemetry } from './shared/lib/telemetry';

const container = document.getElementById('root');
if (!container) throw new Error('Не найден корневой элемент приложения');

createRoot(container).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
);

// Накопленные события отправляются при сворачивании приложения.
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'hidden') void flushTelemetry();
});
