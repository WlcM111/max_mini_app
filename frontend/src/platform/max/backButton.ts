import { getBridge } from './bridge';

type Handler = () => void;

let current: Handler | null = null;

/**
 * Показывает системную кнопку «Назад» MAX и подписывает обработчик (F-17).
 * Подписка одна: при повторном вызове прежний обработчик снимается.
 */
export async function showBackButton(handler: Handler): Promise<void> {
  const bridge = await getBridge();
  const button = bridge.backButton;
  if (!button) return;
  if (current) button.offClick(current);
  current = handler;
  button.onClick(handler);
  button.show();
}

/** Скрывает кнопку «Назад» и снимает обработчик. */
export async function hideBackButton(): Promise<void> {
  const bridge = await getBridge();
  const button = bridge.backButton;
  if (!button) return;
  if (current) button.offClick(current);
  current = null;
  button.hide();
}

/** Доступна ли системная кнопка «Назад» (иначе показываем кнопку в шапке). */
export async function hasSystemBackButton(): Promise<boolean> {
  const bridge = await getBridge();
  return bridge.backButton !== null;
}
