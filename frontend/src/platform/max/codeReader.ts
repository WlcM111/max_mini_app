import { getBridge } from './bridge';
import { track } from '../../shared/lib/telemetry';

export type ScanResult =
  | { kind: 'url'; value: string }
  | { kind: 'number'; value: string }
  | { kind: 'unknown' };

/** Разбирает строку кода по правилу spec §6: ссылка → reference_url, иначе номер. */
export function classifyScan(raw: string): ScanResult {
  const value = raw.trim();
  if (value.startsWith('https://') && value.length <= 1024 && !/\s/.test(value)) {
    return { kind: 'url', value };
  }
  if (value.length > 0 && value.length <= 100) return { kind: 'number', value };
  return { kind: 'unknown' };
}

/** Открывает сканер кода MAX; вызывается синхронно из обработчика клика. */
export async function scanCode(): Promise<ScanResult> {
  const bridge = await getBridge();
  try {
    const raw = await bridge.openCodeReader();
    return classifyScan(raw);
  } catch {
    track({ name: 'bridge_error', code: 'bridge_qr_failed' });
    return { kind: 'unknown' };
  }
}
