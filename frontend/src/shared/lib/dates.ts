import { plural } from './plural';

const DAY_FORMS: [string, string, string] = ['день', 'дня', 'дней'];

/** Проверяет строку календарной даты формата YYYY-MM-DD. */
export function isIsoDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const [y, m, d] = value.split('-').map(Number) as [number, number, number];
  if (m < 1 || m > 12 || d < 1 || d > 31) return false;
  const date = new Date(Date.UTC(y, m - 1, d));
  return date.getUTCFullYear() === y && date.getUTCMonth() === m - 1 && date.getUTCDate() === d;
}

/** Переводит YYYY-MM-DD в DD.MM.YYYY (frontend-architecture §14). */
export function formatDate(value: string | null | undefined): string {
  if (!value || !isIsoDate(value)) return '—';
  const [y, m, d] = value.split('-') as [string, string, string];
  return `${d}.${m}.${y}`;
}

/** Текущая календарная дата в часовом поясе организации. */
export function todayInTimeZone(timeZone: string, now: Date = new Date()): string {
  try {
    return new Intl.DateTimeFormat('en-CA', {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(now);
  } catch {
    return new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit' }).format(now);
  }
}

/** Сдвигает дату на указанное число лет и дней, сохраняя формат YYYY-MM-DD. */
export function shiftDate(value: string, { years = 0, days = 0 }: { years?: number; days?: number }): string {
  if (!isIsoDate(value)) return value;
  const [y, m, d] = value.split('-').map(Number) as [number, number, number];
  const date = new Date(Date.UTC(y + years, m - 1, d + days));
  return date.toISOString().slice(0, 10);
}

/** Текст оставшегося срока: сервер присылает days_left, клиент только оформляет. */
export function daysLeftText(daysLeft: number | null | undefined): string {
  if (daysLeft === null || daysLeft === undefined) return 'бессрочный';
  if (daysLeft > 0) return `осталось ${daysLeft} ${plural(daysLeft, DAY_FORMS)}`;
  if (daysLeft === 0) return 'сегодня последний день';
  const overdue = Math.abs(daysLeft);
  return `просрочен на ${overdue} ${plural(overdue, DAY_FORMS)}`;
}

/** Момент напоминания в часовом поясе организации: DD.MM.YYYY, HH:MM. */
export function formatDateTime(iso: string | null | undefined, timeZone: string): string {
  if (!iso) return '—';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '—';
  try {
    const parts = new Intl.DateTimeFormat('ru-RU', {
      timeZone,
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).formatToParts(date);
    const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
    return `${get('day')}.${get('month')}.${get('year')}, ${get('hour')}:${get('minute')}`;
  } catch {
    return date.toISOString().slice(0, 16).replace('T', ' ');
  }
}
