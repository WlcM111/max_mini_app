/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

// Сборка мини-приложения. Цель es2019 — минимальная поддерживаемая клиентами MAX.
// Прокси /api на локальный edge или core, чтобы dev-сервер работал без CORS.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    target: 'es2019',
    sourcemap: false,
    chunkSizeWarningLimit: 300,
    // CSP запрещает data:-шрифты: ассеты всегда отдельными файлами.
    assetsInlineLimit: 0,
  },
  server: {
    port: 5173,
    proxy: { '/api': { target: 'http://localhost:8085', changeOrigin: true } },
  },
  test: {
    environment: 'jsdom',
    // В тестах fetch выполняется средой Node: базовый адрес должен быть абсолютным
    // и совпадать с origin jsdom, чтобы MSW перехватывал запросы.
    environmentOptions: { jsdom: { url: 'http://localhost:3000' } },
    env: { VITE_API_BASE_URL: 'http://localhost:3000/api/v1' },
    globals: true,
    setupFiles: ['src/test/setup.ts'],
    css: false,
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    exclude: ['e2e/**', 'node_modules/**'],
  },
});
