import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@maxhub/max-ui/dist/styles.css';
import './shared/styles/app.css';
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
