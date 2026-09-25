import { getBridge } from './bridge';

/** Тактильный отклик успеха или ошибки (только iOS и Android, F-25). */
export async function haptic(kind: 'success' | 'error'): Promise<void> {
  const bridge = await getBridge();
  bridge.haptic(kind);
}
