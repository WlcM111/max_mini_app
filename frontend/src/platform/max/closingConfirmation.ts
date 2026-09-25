import { getBridge } from './bridge';

/** Включает подтверждение закрытия, пока в форме есть несохранённые изменения (F-18). */
export async function setClosingConfirmation(enabled: boolean): Promise<void> {
  const bridge = await getBridge();
  if (enabled) bridge.enableClosingConfirmation();
  else bridge.disableClosingConfirmation();
}
