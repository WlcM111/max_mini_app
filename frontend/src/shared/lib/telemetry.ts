// Технические события мини-приложения (FR-17). Отправляются в core-service
// пакетами; сведения о пользователе и строка initData не передаются.

export type TelemetryName = 'bootstrap_completed' | 'bootstrap_failed' | 'bridge_error' | 'api_error_shown';

export type TelemetryCode =
  | 'not_in_max'
  | 'launch_invalid'
  | 'launch_expired'
  | 'network'
  | 'server_5xx'
  | 'bridge_download_failed'
  | 'bridge_share_failed'
  | 'bridge_qr_failed'
  | 'bridge_unsupported'
  | 'render';

export interface TelemetryEvent {
  name: TelemetryName;
  code?: TelemetryCode;
  durationMs?: number;
  platform?: string;
}

interface Payload {
  name: string;
  platform?: string;
  app_version?: string;
  duration_ms?: number;
  code?: string;
}

const MAX_BATCH = 20;
let queue: Payload[] = [];
let timer: ReturnType<typeof setTimeout> | null = null;

function baseUrl(): string {
  return import.meta.env.VITE_API_BASE_URL ?? '/api/v1';
}

function appVersion(): string {
  return import.meta.env.VITE_APP_VERSION ?? 'dev';
}

/** Ставит событие в очередь отправки; сбой отправки не влияет на интерфейс. */
export function track(event: TelemetryEvent): void {
  const payload: Payload = { name: event.name, app_version: appVersion() };
  if (event.code) payload.code = event.code;
  if (typeof event.durationMs === 'number') payload.duration_ms = Math.max(0, Math.round(event.durationMs));
  if (event.platform) payload.platform = event.platform;
  queue.push(payload);
  if (queue.length >= MAX_BATCH) {
    void flushTelemetry();
    return;
  }
  if (timer === null) {
    timer = setTimeout(() => {
      timer = null;
      void flushTelemetry();
    }, 2000);
  }
}

/** Немедленно отправляет накопленные события. */
export async function flushTelemetry(): Promise<void> {
  if (queue.length === 0) return;
  const events = queue.slice(0, MAX_BATCH);
  queue = queue.slice(events.length);
  try {
    await fetch(`${baseUrl()}/client-events`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ events }),
      keepalive: true,
    });
  } catch {
    /* телеметрия необязательна: теряем событие молча */
  }
}

/** Очищает очередь: используется тестами. */
export function resetTelemetry(): void {
  queue = [];
  if (timer !== null) {
    clearTimeout(timer);
    timer = null;
  }
}
