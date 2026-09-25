import { defineConfig, devices } from '@playwright/test';

// Браузерные сценарии выполняются против локального стенда: собранный интерфейс
// отдаёт edge (Caddy) на http://localhost:8080, API проксируется туда же.
// Запуск: npm run e2e (требуются загруженные браузеры Playwright).
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:8085';

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL,
    locale: 'ru-RU',
    timezoneId: 'Europe/Moscow',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'mobile-android', use: { ...devices['Pixel 7'] } },
    { name: 'mobile-ios', use: { ...devices['iPhone 13'] } },
    { name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 420, height: 900 } } },
  ],
});
