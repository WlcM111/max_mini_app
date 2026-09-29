import { isIsoDate } from './dates';

export const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'];
export const MONTHS_GEN = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'];
export const MONTHS_SHORT = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
export const WEEKDAYS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
const WEEKDAY_NAMES = ['воскресенье', 'понедельник', 'вторник', 'среда', 'четверг', 'пятница', 'суббота'];

const pad = (value: number) => String(value).padStart(2, '0');

/** Месяц YYYY-MM, сдвинутый на delta месяцев. */
export function shiftMonth(month: string, delta: number): string {
  const [year = 2026, index = 1] = month.split('-').map(Number);
  const date = new Date(Date.UTC(year, index - 1 + delta, 1));
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}`;
}

/** Ячейки месяца по неделям с понедельника; null — пустые клетки до первого и после последнего дня. */
export function monthCells(month: string): (string | null)[] {
  const [year = 2026, index = 1] = month.split('-').map(Number);
  const lead = (new Date(Date.UTC(year, index - 1, 1)).getUTCDay() + 6) % 7;
  const length = new Date(Date.UTC(year, index, 0)).getUTCDate();
  const cells: (string | null)[] = [
    ...Array.from({ length: lead }, () => null),
    ...Array.from({ length }, (_, day) => `${month}-${pad(day + 1)}`),
  ];
  while (cells.length % 7 !== 0) cells.push(null);
  return cells;
}

/** Дата YYYY-MM-DD, сдвинутая на days дней. */
export function addDays(iso: string, days: number): string {
  const [year = 2026, month = 1, day = 1] = iso.split('-').map(Number);
  return new Date(Date.UTC(year, month - 1, day + days)).toISOString().slice(0, 10);
}

/** Сегодняшняя дата устройства в формате YYYY-MM-DD. */
export function localToday(now: Date = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/** Дата, введённая вручную: 31.12.2026, 31/12/2026, 31122026 или 2026-12-31. */
export function parseTypedDate(text: string): string | null {
  const value = text.trim();
  let match = /^(\d{1,2})[./-](\d{1,2})[./-](\d{4})$/.exec(value) ?? /^(\d{2})(\d{2})(\d{4})$/.exec(value);
  if (match) {
    const iso = `${match[3]}-${pad(Number(match[2]))}-${pad(Number(match[1]))}`;
    return isIsoDate(iso) ? iso : null;
  }
  match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  return match && isIsoDate(value) ? value : null;
}

/** Маска ручного ввода: цифры расставляются как ДД.ММ.ГГГГ. */
export function maskTypedDate(text: string): string {
  if (/^\d{4}-/.test(text)) return text.slice(0, 10); // YYYY-MM-DD вставкой
  const digits = text.replace(/\D/g, '').slice(0, 8);
  if (digits.length <= 2) return digits;
  if (digits.length <= 4) return `${digits.slice(0, 2)}.${digits.slice(2)}`;
  return `${digits.slice(0, 2)}.${digits.slice(2, 4)}.${digits.slice(4)}`;
}

/** Подпись дня для экранного диктора: «13 марта 2029, вторник». */
export function dayLabel(iso: string): string {
  const [year = 2026, month = 1, day = 1] = iso.split('-').map(Number);
  const weekday = WEEKDAY_NAMES[new Date(Date.UTC(year, month - 1, day)).getUTCDay()] ?? '';
  return `${day} ${MONTHS_GEN[month - 1] ?? ''} ${year}, ${weekday}`;
}
