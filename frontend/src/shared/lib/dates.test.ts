import { describe, expect, it } from 'vitest';
import { daysLeftText, formatDate, formatDateTime, isIsoDate, shiftDate, todayInTimeZone } from './dates';
import { plural, pluralWithCount } from './plural';

describe('T-FE-UTIL: даты и склонения', () => {
  it('проверяет формат календарной даты', () => {
    expect(isIsoDate('2026-09-24')).toBe(true);
    expect(isIsoDate('2026-02-30')).toBe(false);
    expect(isIsoDate('24.09.2026')).toBe(false);
    expect(isIsoDate('2026-13-01')).toBe(false);
  });

  it('печатает дату в формате DD.MM.YYYY', () => {
    expect(formatDate('2026-09-24')).toBe('24.09.2026');
    expect(formatDate(null)).toBe('—');
    expect(formatDate('мусор')).toBe('—');
  });

  it('определяет сегодняшнюю дату в часовом поясе организации', () => {
    const moment = new Date('2026-09-24T21:30:00Z');
    expect(todayInTimeZone('Asia/Vladivostok', moment)).toBe('2026-09-25');
    expect(todayInTimeZone('Europe/Moscow', moment)).toBe('2026-09-25');
    expect(todayInTimeZone('UTC', moment)).toBe('2026-09-24');
  });

  it('сдвигает дату на год и на день', () => {
    expect(shiftDate('2026-09-24', { years: 1 })).toBe('2027-09-24');
    expect(shiftDate('2026-12-31', { days: 1 })).toBe('2027-01-01');
    expect(shiftDate('2024-02-29', { years: 1 })).toBe('2025-03-01');
  });

  it('формирует текст оставшегося срока', () => {
    expect(daysLeftText(null)).toBe('бессрочный');
    expect(daysLeftText(0)).toBe('сегодня последний день');
    expect(daysLeftText(1)).toBe('осталось 1 день');
    expect(daysLeftText(3)).toBe('осталось 3 дня');
    expect(daysLeftText(11)).toBe('осталось 11 дней');
    expect(daysLeftText(21)).toBe('осталось 21 день');
    expect(daysLeftText(-1)).toBe('просрочен на 1 день');
    expect(daysLeftText(-54)).toBe('просрочен на 54 дня');
  });

  it('склоняет существительные после числа', () => {
    expect(plural(1, ['день', 'дня', 'дней'])).toBe('день');
    expect(plural(2, ['день', 'дня', 'дней'])).toBe('дня');
    expect(plural(5, ['день', 'дня', 'дней'])).toBe('дней');
    expect(plural(12, ['день', 'дня', 'дней'])).toBe('дней');
    expect(pluralWithCount(30, ['день', 'дня', 'дней'])).toBe('30 дней');
  });

  it('показывает момент напоминания в поясе организации', () => {
    expect(formatDateTime('2026-09-25T06:00:00Z', 'Europe/Moscow')).toBe('25.09.2026, 09:00');
    expect(formatDateTime(null, 'Europe/Moscow')).toBe('—');
  });
});
