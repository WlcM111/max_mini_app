import { readStorage, removeStorage, writeStorage } from '../../shared/lib/storage';

// Чек-лист подготовки к продлению (FR-20). Отметки шагов — личная пометка
// пользователя на его устройстве: они не меняют предметные данные, поэтому
// хранятся в localStorage и не отправляются на сервер. Потеря отметок безопасна:
// сами шаги приходят из справочника в поле renewal_steps.

const KEY_PREFIX = 'vovremya.renewal.';

/** Ключ хранения отметок для конкретного документа. */
export function checklistKey(documentId: string): string {
  return `${KEY_PREFIX}${documentId}`;
}

/** Приводит список отметок к корректному виду: только существующие шаги, без повторов. */
export function normalizeChecklist(value: unknown, stepCount: number): number[] {
  if (!Array.isArray(value)) return [];
  const unique = new Set<number>();
  for (const item of value) {
    if (typeof item !== 'number' || !Number.isInteger(item)) continue;
    if (item < 0 || item >= stepCount) continue;
    unique.add(item);
  }
  return Array.from(unique).sort((a, b) => a - b);
}

/** Читает отметки документа; повреждённое значение считается пустым. */
export function readChecklist(documentId: string, stepCount: number): number[] {
  const raw = readStorage('local', checklistKey(documentId));
  if (!raw) return [];
  try {
    return normalizeChecklist(JSON.parse(raw), stepCount);
  } catch {
    removeStorage('local', checklistKey(documentId));
    return [];
  }
}

/** Сохраняет отметки; пустой список удаляет запись, чтобы не копить ключи. */
export function saveChecklist(documentId: string, done: number[]): void {
  if (done.length === 0) {
    removeStorage('local', checklistKey(documentId));
    return;
  }
  writeStorage('local', checklistKey(documentId), JSON.stringify(done));
}

/** Переключает отметку шага. */
export function toggleStep(done: number[], index: number): number[] {
  const next = done.includes(index) ? done.filter((item) => item !== index) : [...done, index];
  return next.sort((a, b) => a - b);
}

/** Снимает все отметки документа. */
export function clearChecklist(documentId: string): void {
  removeStorage('local', checklistKey(documentId));
}

/** Текст прогресса для заголовка раздела. */
export function progressText(done: number, total: number): string {
  if (total === 0) return 'Шагов нет';
  if (done === 0) return `Шагов: ${total}`;
  if (done >= total) return 'Всё готово';
  return `Готово ${done} из ${total}`;
}
