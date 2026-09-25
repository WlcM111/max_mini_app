// Контракт адаптера клиентского SDK MAX (handoff frontend §8).
// Единственное место проекта, где допускается обращение к window.WebApp, —
// каталог src/platform/max. Все возможности подтверждены docs/max/max-platform-facts.md.

export type Platform = 'ios' | 'android' | 'desktop' | 'web';

export interface BackButtonApi {
  show(): void;
  hide(): void;
  onClick(cb: () => void): void;
  offClick(cb: () => void): void;
}

export interface MaxBridge {
  readonly kind: 'max' | 'mock' | 'unavailable';
  readonly initData: string;
  readonly platform: Platform;
  readonly version: string;
  readonly backButton: BackButtonApi | null;
  enableClosingConfirmation(): void;
  disableClosingConfirmation(): void;
  openLink(url: string): Promise<void>;
  openMaxLink(url: string): Promise<void>;
  shareMaxContent(p: { text: string; link: string }): Promise<void>;
  downloadFile(url: string, fileName: string): Promise<void>;
  readonly canScanQr: boolean;
  openCodeReader(): Promise<string>;
  haptic(kind: 'success' | 'error'): void;
}

/** Ошибка SDK MAX: методы отклоняют промис объектом { error: { code } } (F-27). */
export class BridgeError extends Error {
  constructor(readonly code: string) {
    super(`MAX Bridge: ${code}`);
    this.name = 'BridgeError';
  }
}

/** Приводит отказ Bridge к BridgeError с кодом. */
export function toBridgeError(reason: unknown): BridgeError {
  if (reason instanceof BridgeError) return reason;
  if (typeof reason === 'object' && reason !== null) {
    const holder = reason as { error?: { code?: unknown }; code?: unknown };
    const code = holder.error?.code ?? holder.code;
    if (typeof code === 'string') return new BridgeError(code);
  }
  return new BridgeError('unknown');
}
