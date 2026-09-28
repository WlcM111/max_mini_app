import { plural } from './plural';

/** Формы слова «день» для plural(). */
export const DAY_FORMS: [string, string, string] = ['день', 'дня', 'дней'];

const MONTHS = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек'];

/** Части даты для листка календаря: число, месяц, год. */
export function leafParts(iso: string): { day: string; month: string; year: string } | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!match) return null;
  return { year: match[1] ?? '', month: MONTHS[Number(match[2]) - 1] ?? '', day: String(Number(match[3])) };
}

/** Короткая подпись к сроку в строке списка: «через 12 дней», «3 дня назад». */
export function relativeDays(daysLeft: number | null | undefined): string {
  if (daysLeft === null || daysLeft === undefined) return '';
  if (daysLeft === 0) return 'истекает сегодня';
  const count = Math.abs(daysLeft);
  const words = `${count} ${plural(count, DAY_FORMS)}`;
  return daysLeft > 0 ? `через ${words}` : `${words} назад`;
}

/** Крупное число в карточке документа и подпись к нему (без слов статуса). */
export function heroCount(daysLeft: number | null | undefined): { value: string; unit: string; word: boolean } {
  if (daysLeft === null || daysLeft === undefined) return { value: '', unit: 'Срок окончания не установлен', word: false };
  if (daysLeft === 0) return { value: 'Сегодня', unit: 'последний день срока', word: true };
  const count = Math.abs(daysLeft);
  const unit = daysLeft > 0 ? `${plural(count, DAY_FORMS)} до окончания срока` : `${plural(count, DAY_FORMS)} просрочки`;
  return { value: String(count), unit, word: false };
}

/** Доля прошедшего периода действия в процентах (0…100). */
export function periodProgress(from: string, until: string, today: string): number | null {
  const start = Date.parse(`${from}T00:00:00Z`);
  const end = Date.parse(`${until}T00:00:00Z`);
  const now = Date.parse(`${today}T00:00:00Z`);
  if (Number.isNaN(start) || Number.isNaN(end) || Number.isNaN(now) || end <= start) return null;
  return Math.min(100, Math.max(0, ((now - start) / (end - start)) * 100));
}

/** Инициалы для аватара. */
export function initials(first: string, last?: string | null): string {
  const value = `${first.trim().charAt(0)}${(last ?? '').trim().charAt(0)}`.toUpperCase();
  return value === '' ? '?' : value;
}

/** Первая буква названия без кавычек и знаков. */
export function nameInitial(name: string): string {
  const match = /[0-9A-Za-zА-Яа-яЁё]/.exec(name);
  return match ? match[0].toUpperCase() : '?';
}

/** Устойчивый номер цвета аватара по строке (0…5). */
export function toneIndex(seed: string): number {
  let hash = 0;
  for (let index = 0; index < seed.length; index += 1) hash = (hash * 31 + seed.charCodeAt(index)) >>> 0;
  return hash % 6;
}

export const ROLE_TITLES: Record<string, string> = { owner: 'Владелец', editor: 'Редактор', viewer: 'Наблюдатель' };

const PLATFORM_TITLES: Record<string, string> = {
  ios: 'iOS',
  android: 'Android',
  desktop: 'MAX для компьютера',
  web: 'Веб-версия MAX',
};

export function platformTitle(platform: string): string {
  return PLATFORM_TITLES[platform] ?? platform;
}

const ZONE_CITIES: Record<string, string> = {
  'Europe/Kaliningrad': 'Калининград',
  'Europe/Moscow': 'Москва',
  'Europe/Samara': 'Самара',
  'Asia/Yekaterinburg': 'Екатеринбург',
  'Asia/Omsk': 'Омск',
  'Asia/Novosibirsk': 'Новосибирск',
  'Asia/Krasnoyarsk': 'Красноярск',
  'Asia/Irkutsk': 'Иркутск',
  'Asia/Yakutsk': 'Якутск',
  'Asia/Vladivostok': 'Владивосток',
  'Asia/Magadan': 'Магадан',
  'Asia/Kamchatka': 'Камчатка',
};

/** Понятная подпись часового пояса: «Москва (GMT+3)». */
export function timezoneLabel(zone: string, now: Date = new Date()): string {
  const city = ZONE_CITIES[zone] ?? zone;
  try {
    const offset = new Intl.DateTimeFormat('ru-RU', { timeZone: zone, timeZoneName: 'shortOffset' })
      .formatToParts(now)
      .find((part) => part.type === 'timeZoneName')?.value;
    return offset ? `${city} (${offset})` : city;
  } catch {
    return city;
  }
}
