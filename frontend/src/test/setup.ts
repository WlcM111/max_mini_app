import '@testing-library/jest-dom/vitest';
import { afterAll, afterEach, beforeAll } from 'vitest';
import { cleanup } from '@testing-library/react';
import { server } from './msw/server';
import { calls } from './msw/handlers';
import { resetTelemetry } from '../shared/lib/telemetry';
import { clearSession, setLaunchContext } from '../session/sessionStore';
import { resetBridge } from '../platform/max/bridge';

// jsdom не реализует matchMedia, а компоненты MAX UI и тема на него опираются.
if (typeof window !== 'undefined' && typeof window.matchMedia !== 'function') {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

// Базовый адрес API в тестах совпадает с рабочим: запросы перехватывает MSW.
beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' });
});

afterEach(() => {
  server.resetHandlers();
  cleanup();
  calls.reset();
  resetTelemetry();
  clearSession();
  resetBridge();
  setLaunchContext({ initData: 'auth_date=1&user=%7B%7D&hash=test', platform: 'web', appVersion: 'test' });
  window.sessionStorage.clear();
  window.localStorage.clear();
});

afterAll(() => {
  server.close();
});
