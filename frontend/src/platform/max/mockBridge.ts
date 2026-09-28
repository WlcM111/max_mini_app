import type { MaxBridge } from './types';

// Имитация окружения MAX для локальной разработки (handoff frontend §8).
// Модуль подключается динамическим import() только при VITE_MOCK_BRIDGE === 'true',
// поэтому в prod-сборку он не попадает. Настоящих секретов здесь нет: используется
// локальный тестовый токен бота из .env.development.

const encoder = new TextEncoder();

async function hmacSha256(key: ArrayBuffer | Uint8Array, message: string): Promise<Uint8Array> {
  const material = key instanceof Uint8Array ? new Uint8Array(key) : new Uint8Array(key);
  const cryptoKey = await crypto.subtle.importKey('raw', material, { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
  const signature = await crypto.subtle.sign('HMAC', cryptoKey, encoder.encode(message));
  return new Uint8Array(signature);
}

function toHex(bytes: Uint8Array): string {
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

/** Собирает подписанную строку данных запуска по алгоритму spec §4. */
export async function signInitData(params: Record<string, string>, botToken: string): Promise<string> {
  const keys = Object.keys(params).sort();
  const checkString = keys.map((k) => `${k}=${params[k] ?? ''}`).join('\n');
  const secret = await hmacSha256(encoder.encode('WebAppData'), botToken);
  const hash = toHex(await hmacSha256(secret, checkString));
  const query = keys.map((k) => `${k}=${encodeURIComponent(params[k] ?? '')}`).join('&');
  return `${query}&hash=${hash}`;
}

function mockBackButton(): MaxBridge['backButton'] {
  let handler: (() => void) | null = null;
  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = '‹ Назад (MAX)';
  button.className = 'mock-back-button';
  button.hidden = true;
  button.addEventListener('click', () => handler?.());
  document.body.appendChild(button);
  return {
    show: () => {
      button.hidden = false;
    },
    hide: () => {
      button.hidden = true;
    },
    onClick: (cb) => {
      handler = cb;
    },
    offClick: (cb) => {
      if (handler === cb) handler = null;
    },
  };
}

function showMockBanner(): void {
  if (document.querySelector('.mock-banner')) return;
  const banner = document.createElement('div');
  banner.className = 'mock-banner';
  banner.textContent = 'Имитация MAX';
  document.body.prepend(banner);
  document.documentElement.classList.add('has-mock-bar');
}

/** Создаёт имитацию Bridge: подписывает initData и подменяет системные действия. */
export async function createMockBridge(): Promise<MaxBridge> {
  const search = new URLSearchParams(window.location.search);
  const userId = Number(search.get('mockUser') ?? '1001');
  const startParam = search.get('startapp') ?? '';
  const token = import.meta.env.VITE_MOCK_BOT_TOKEN ?? 'devonly-local-bot-token';
  const params: Record<string, string> = {
    auth_date: String(Math.floor(Date.now() / 1000)),
    query_id: `mock-${userId}-${Date.now()}`,
    user: JSON.stringify({
      id: userId,
      first_name: `Тест ${userId}`,
      last_name: null,
      username: null,
      language_code: 'ru',
      photo_url: null,
    }),
  };
  if (startParam) params.start_param = startParam;
  const initData = await signInitData(params, token);

  showMockBanner();
  const backButton = mockBackButton();
  let confirmClosing = false;
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (!confirmClosing) return;
    event.preventDefault();
    event.returnValue = '';
  };
  window.addEventListener('beforeunload', beforeUnload);

  return {
    kind: 'mock',
    initData,
    platform: 'web',
    version: 'mock',
    backButton,
    enableClosingConfirmation: () => {
      confirmClosing = true;
    },
    disableClosingConfirmation: () => {
      confirmClosing = false;
    },
    openLink: async (url) => {
      window.open(url, '_blank', 'noopener,noreferrer');
    },
    openMaxLink: async (url) => {
      window.open(url, '_blank', 'noopener,noreferrer');
    },
    shareMaxContent: async ({ text, link }) => {
      window.prompt('Имитация шеринга MAX — скопируйте ссылку', `${text} ${link}`);
    },
    downloadFile: async (url) => {
      window.open(url, '_blank', 'noopener,noreferrer');
    },
    canScanQr: true,
    openCodeReader: async () => window.prompt('Имитация сканера: введите содержимое кода') ?? '',
    haptic: () => undefined,
  };
}
