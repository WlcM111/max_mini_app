import { getBridge } from './bridge';
import { copyToClipboard } from './links';
import { toBridgeError } from './types';
import { track } from '../../shared/lib/telemetry';

export type ShareOutcome = 'shared' | 'copied' | 'failed';

/**
 * Делится ссылкой приглашения: сначала экран шеринга MAX (F-22), затем диплинк
 * https://max.ru/:share (F-07), затем копирование в буфер обмена.
 */
export async function shareInvite(text: string, link: string): Promise<ShareOutcome> {
  const bridge = await getBridge();
  try {
    await bridge.shareMaxContent({ text, link });
    return 'shared';
  } catch (reason) {
    track({ name: 'bridge_error', code: 'bridge_share_failed' });
    const fallback = `https://max.ru/:share?text=${encodeURIComponent(`${text} ${link}`)}`;
    if (bridge.platform !== 'desktop') {
      try {
        await bridge.openMaxLink(fallback);
        return 'shared';
      } catch {
        // переходим к копированию
      }
    }
    const copied = await copyToClipboard(link);
    if (!copied) toBridgeError(reason);
    return copied ? 'copied' : 'failed';
  }
}
