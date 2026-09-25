import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactHooks from 'eslint-plugin-react-hooks';

// Линтер: строгие правила TypeScript, правила хуков React и запрет опасного HTML.
export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'src/api/schema.d.ts', 'playwright-report', 'test-results'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    plugins: { 'react-hooks': reactHooks },
    languageOptions: {
      ecmaVersion: 2022,
      globals: {
        window: 'readonly', document: 'readonly', navigator: 'readonly', console: 'readonly',
        fetch: 'readonly', localStorage: 'readonly', sessionStorage: 'readonly',
        setTimeout: 'readonly', clearTimeout: 'readonly', crypto: 'readonly',
        performance: 'readonly', AbortController: 'readonly', HTMLElement: 'readonly',
        HTMLInputElement: 'readonly', HTMLTextAreaElement: 'readonly', HTMLSelectElement: 'readonly',
        TextEncoder: 'readonly', URLSearchParams: 'readonly', URL: 'readonly', Response: 'readonly',
        Request: 'readonly', Headers: 'readonly', MediaQueryListEvent: 'readonly', Event: 'readonly',
        process: 'readonly', __dirname: 'readonly', globalThis: 'readonly',
      },
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/consistent-type-imports': 'off',
      'no-restricted-properties': [
        'error',
        { object: 'window', property: 'WebApp', message: 'Обращение к window.WebApp только в src/platform/max' },
      ],
      'no-restricted-syntax': [
        'error',
        {
          selector: 'JSXAttribute[name.name="dangerouslySetInnerHTML"]',
          message: 'dangerouslySetInnerHTML запрещён (frontend-architecture §13)',
        },
      ],
    },
  },
  {
    files: ['src/platform/max/**/*.ts'],
    rules: { 'no-restricted-properties': 'off' },
  },
  {
    files: ['src/test/**/*.ts', '**/*.test.ts', '**/*.test.tsx', 'e2e/**/*.ts'],
    rules: { '@typescript-eslint/no-explicit-any': 'off' },
  },
);
