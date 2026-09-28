import { defineConfig, devices } from '@playwright/test';

// Сравнение скриншотов: собранный фронтенд в режиме имитации MAX + подмена API фикстурами.
// Эталоны лежат в e2e/visual/__screenshots__ и обновляются командой: npm run visual:update
export default defineConfig({
  testDir: './e2e/visual',
  snapshotPathTemplate: '{testDir}/__screenshots__/{projectName}/{arg}{ext}',
  fullyParallel: true,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['html', { open: 'never' }], ['list']] : 'list',
  use: { baseURL: 'http://localhost:4173', locale: 'ru-RU', timezoneId: 'Europe/Moscow' },
  webServer: {
    command: 'npm run build && npx vite preview --port 4173 --strictPort',
    url: 'http://localhost:4173',
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    env: { VITE_MOCK_BRIDGE: 'true' },
  },
  projects: [
    { name: 'phone-320', use: { ...devices['iPhone SE'], viewport: { width: 320, height: 640 } } },
    { name: 'phone-390', use: { ...devices['iPhone 13'] } },
    { name: 'tablet-820', use: { viewport: { width: 820, height: 1180 } } },
    { name: 'desktop-1280', use: { viewport: { width: 1280, height: 820 } } },
  ],
});
