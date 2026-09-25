import { BridgeError, toBridgeError, type MaxBridge, type Platform } from './types';
import { track } from '../../shared/lib/telemetry';

// Форма объекта window.WebApp, которую использует приложение (F-08…F-25).
// Описаны только фактически вызываемые методы; остальные не используются.
interface WebAppBackButton {
  show?: () => void;
  hide?: () => void;
  onClick?: (cb: () => void) => void;
  offClick?: (cb: () => void) => void;
}

interface WebAppLike {
  initData?: string;
  platform?: string;
  version?: string;
  BackButton?: WebAppBackButton;
  HapticFeedback?: { notificationOccurred?: (kind: string) => void };
  enableClosingConfirmation?: () => void;
  disableClosingConfirmation?: () => void;
  openLink?: (url: string) => Promise<void> | void;
  openMaxLink?: (url: string) => Promise<void> | void;
  shareMaxContent?: (p: { text: string; link: string }) => Promise<void> | void;
  downloadFile?: (url: string, fileName: string) => Promise<void> | void;
  openCodeReader?: (fileSelect?: boolean) => Promise<string> | string;
}

function readWebApp(): WebAppLike | null {
  if (typeof window === 'undefined') return null;
  const candidate = (window as unknown as { WebApp?: WebAppLike }).WebApp;
  return candidate && typeof candidate === 'object' ? candidate : null;
}

function normalizePlatform(value: unknown): Platform {
  return value === 'ios' || value === 'android' || value === 'desktop' || value === 'web' ? value : 'web';
}

async function invoke(fn: (() => Promise<unknown> | unknown) | undefined, name: string): Promise<void> {
  if (!fn) throw new BridgeError(`client.${name}.unsupported`);
  try {
    await fn();
  } catch (reason) {
    throw toBridgeError(reason);
  }
}

/** Адаптер реального SDK MAX. */
function realBridge(webApp: WebAppLike, initData: string): MaxBridge {
  const platform = normalizePlatform(webApp.platform);
  const backButton = webApp.BackButton
    ? {
        show: () => webApp.BackButton?.show?.(),
        hide: () => webApp.BackButton?.hide?.(),
        onClick: (cb: () => void) => webApp.BackButton?.onClick?.(cb),
        offClick: (cb: () => void) => webApp.BackButton?.offClick?.(cb),
      }
    : null;
  return {
    kind: 'max',
    initData,
    platform,
    version: typeof webApp.version === 'string' ? webApp.version : 'unknown',
    backButton,
    enableClosingConfirmation: () => webApp.enableClosingConfirmation?.(),
    disableClosingConfirmation: () => webApp.disableClosingConfirmation?.(),
    openLink: (url) => invoke(() => webApp.openLink?.(url), 'open_link'),
    openMaxLink: (url) => invoke(() => webApp.openMaxLink?.(url), 'open_max_link'),
    shareMaxContent: (p) => invoke(() => webApp.shareMaxContent?.(p), 'share_max_content'),
    downloadFile: (url, fileName) => invoke(() => webApp.downloadFile?.(url, fileName), 'download_file'),
    canScanQr: typeof webApp.openCodeReader === 'function',
    openCodeReader: async () => {
      if (!webApp.openCodeReader) throw new BridgeError('client.open_code_reader.unsupported');
      try {
        // Аргумент fileSelect=true разрешает выбор изображения с кодом (F-23).
        return await webApp.openCodeReader(true);
      } catch (reason) {
        throw toBridgeError(reason);
      }
    },
    // HapticFeedback недоступен в desktop и web клиентах (F-25).
    haptic: (kind) => {
      if (platform !== 'ios' && platform !== 'android') return;
      webApp.HapticFeedback?.notificationOccurred?.(kind);
    },
  };
}

/** Приложение открыто вне MAX: доступных возможностей нет. */
export function unavailableBridge(): MaxBridge {
  const fail = () => Promise.reject(new BridgeError('client.environment.unavailable'));
  return {
    kind: 'unavailable',
    initData: '',
    platform: 'web',
    version: 'unknown',
    backButton: null,
    enableClosingConfirmation: () => undefined,
    disableClosingConfirmation: () => undefined,
    openLink: fail,
    openMaxLink: fail,
    shareMaxContent: fail,
    downloadFile: fail,
    canScanQr: false,
    openCodeReader: () => Promise.reject(new BridgeError('client.environment.unavailable')),
    haptic: () => undefined,
  };
}

let cached: Promise<MaxBridge> | null = null;

/**
 * Возвращает адаптер SDK: настоящий MAX, имитацию (только при VITE_MOCK_BRIDGE=true)
 * либо признак недоступного окружения. Результат кэшируется на время жизни страницы.
 */
export function getBridge(): Promise<MaxBridge> {
  if (!cached) cached = resolveBridge();
  return cached;
}

/** Сбрасывает кэш адаптера: используется тестами. */
export function resetBridge(): void {
  cached = null;
}

async function resolveBridge(): Promise<MaxBridge> {
  const webApp = readWebApp();
  const initData = typeof webApp?.initData === 'string' ? webApp.initData : '';
  if (webApp && initData.length > 0) return realBridge(webApp, initData);
  if (import.meta.env.VITE_MOCK_BRIDGE === 'true') {
    const { createMockBridge } = await import('./mockBridge');
    return createMockBridge();
  }
  track({ name: 'bootstrap_failed', code: 'not_in_max' });
  return unavailableBridge();
}
