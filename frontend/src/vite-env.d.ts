/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_MOCK_BRIDGE?: string;
  readonly VITE_MOCK_BOT_TOKEN?: string;
  readonly VITE_APP_VERSION?: string;
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_MSW?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
