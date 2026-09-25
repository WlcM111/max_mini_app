import { defineConfig } from 'vitest/config';
import { fileURLToPath, URL } from 'node:url';

// Интеграционные тесты выполняются против НАСТОЯЩИХ трёх микросервисов.
// Стенд поднимается скриптом scripts/run_local_stack.sh, MSW не используется.
export default defineConfig({
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  test: {
    environment: 'node',
    globals: true,
    include: ['test/integration/**/*.int.test.ts'],
    testTimeout: 30_000,
    hookTimeout: 30_000,
    fileParallelism: false,
    env: {
      VITE_API_BASE_URL: process.env.VOVREMYA_API_BASE_URL ?? 'http://127.0.0.1:18170/api/v1',
      VITE_APP_VERSION: 'integration',
    },
  },
});
