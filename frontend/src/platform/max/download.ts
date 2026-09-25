import { getBridge } from './bridge';
import { track } from '../../shared/lib/telemetry';

export type DownloadOutcome = 'downloaded' | 'opened' | 'failed';

/**
 * Скачивает файл календаря. Метод downloadFile работает только внутри клиента MAX
 * (F-20); в вебе и при ошибке ссылка открывается через openLink.
 */
export async function downloadCalendar(url: string, fileName: string): Promise<DownloadOutcome> {
  const bridge = await getBridge();
  if (bridge.kind === 'max' && bridge.platform !== 'web') {
    try {
      await bridge.downloadFile(url, fileName);
      return 'downloaded';
    } catch {
      track({ name: 'bridge_error', code: 'bridge_download_failed' });
    }
  }
  try {
    await bridge.openLink(url);
    return 'opened';
  } catch {
    track({ name: 'bridge_error', code: 'bridge_download_failed' });
    return 'failed';
  }
}
