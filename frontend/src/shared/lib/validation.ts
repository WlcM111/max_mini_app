import { isIsoDate } from './dates';

// Правила совпадают с OpenAPI и frontend-architecture §8. Клиентская проверка
// не заменяет серверную: она только ускоряет обратную связь пользователю.

export const OFFSET_PRESETS = [90, 60, 30, 14, 7, 3, 1, 0] as const;
export const MAX_OFFSETS = 5;

export function validateOrganizationName(value: string): string | null {
  const trimmed = value.trim();
  if (trimmed.length === 0 || trimmed.length > 100) return 'Введите название (до 100 символов)';
  return null;
}

export function validateRequiredCode(value: string): string | null {
  return value.trim().length === 0 ? 'Выберите значение' : null;
}

export function validateTimezone(value: string): string | null {
  return value.trim().length === 0 ? 'Выберите часовой пояс' : null;
}

export function validateDocumentTitle(value: string): string | null {
  const trimmed = value.trim();
  if (trimmed.length === 0 || trimmed.length > 200) return 'Введите название (до 200 символов)';
  return null;
}

export function validateMaxLength(value: string, max: number, message: string): string | null {
  return value.length > max ? message : null;
}

export function validateReferenceUrl(value: string): string | null {
  if (value.trim().length === 0) return null;
  if (!/^https:\/\/\S+$/.test(value) || value.length > 1024) return 'Ссылка должна начинаться с https://';
  return null;
}

export function validateDates(validFrom: string | null, validUntil: string | null): string | null {
  if (validFrom && !isIsoDate(validFrom)) return 'Проверьте дату начала';
  if (validUntil && !isIsoDate(validUntil)) return 'Проверьте дату окончания';
  if (validFrom && validUntil && validUntil < validFrom) return 'Дата окончания раньше даты начала';
  return null;
}

export function validateOffsets(offsets: number[]): string | null {
  if (offsets.length > MAX_OFFSETS) return 'Не более 5 напоминаний';
  if (new Set(offsets).size !== offsets.length) return 'Значения не должны повторяться';
  if (offsets.some((value) => !Number.isInteger(value) || value < 0 || value > 365)) {
    return 'Напоминание — от 0 до 365 дней';
  }
  return null;
}

/** Время напоминаний: шаг 30 минут в промежутке 06:00–22:00. */
export function validateNotifyTime(value: string): string | null {
  if (!/^\d{2}:\d{2}$/.test(value)) return 'Укажите время в формате ЧЧ:ММ';
  const [hours, minutes] = value.split(':').map(Number) as [number, number];
  if (minutes !== 0 && minutes !== 30) return 'Время выбирается с шагом 30 минут';
  if (hours < 6 || hours > 22 || (hours === 22 && minutes > 0)) return 'Выберите время с 06:00 до 22:00';
  return null;
}

/** Список допустимых значений времени напоминаний. */
export function notifyTimeOptions(): string[] {
  const options: string[] = [];
  for (let hour = 6; hour <= 22; hour += 1) {
    options.push(`${String(hour).padStart(2, '0')}:00`);
    if (hour < 22) options.push(`${String(hour).padStart(2, '0')}:30`);
  }
  return options;
}
