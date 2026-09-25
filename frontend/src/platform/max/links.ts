import { getBridge } from './bridge';
import { toBridgeError } from './types';
import { track } from '../../shared/lib/telemetry';

/** Разрешены только абсолютные https-ссылки (frontend-architecture §13). */
export function isSafeExternalUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

/** Открывает внешнюю ссылку через Bridge; вызывается синхронно из обработчика клика. */
export async function openExternal(url: string): Promise<void> {
  if (!isSafeExternalUrl(url)) throw new Error('Ссылка должна начинаться с https://');
  const bridge = await getBridge();
  try {
    await bridge.openLink(url);
  } catch (reason) {
    const error = toBridgeError(reason);
    track({ name: 'bridge_error', code: 'bridge_unsupported' });
    if (bridge.kind === 'max') window.open(url, '_blank', 'noopener,noreferrer');
    else throw error;
  }
}

/** Открывает диплинк MAX (чат с ботом, экран шеринга). */
export async function openMaxDeepLink(url: string): Promise<void> {
  const bridge = await getBridge();
  try {
    await bridge.openMaxLink(url);
  } catch (reason) {
    track({ name: 'bridge_error', code: 'bridge_unsupported' });
    await copyToClipboard(url);
    throw toBridgeError(reason);
  }
}

/** Копирует текст в буфер обмена; запасной путь для шеринга и ссылок. */
export async function copyToClipboard(value: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    return false;
  }
}
