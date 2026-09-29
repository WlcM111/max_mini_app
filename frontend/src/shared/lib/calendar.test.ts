import { describe, expect, it } from 'vitest';
import { addDays, dayLabel, localToday, maskTypedDate, monthCells, parseTypedDate, shiftMonth } from './calendar';

describe('календарь: вспомогательные функции', () => {
  it('сдвигает месяц через границу года', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01');
    expect(shiftMonth('2026-01', -1)).toBe('2025-12');
    expect(shiftMonth('2026-03', 12)).toBe('2027-03');
  });

  it('строит сетку месяца с понедельника', () => {
    const cells = monthCells('2026-09'); // 1 сентября 2026 — вторник
    expect(cells[0]).toBeNull();
    expect(cells[1]).toBe('2026-09-01');
    expect(cells.length % 7).toBe(0);
    expect(cells.filter((day) => day !== null)).toHaveLength(30);
  });

  it('сдвигает дату на дни с переходом месяца и года', () => {
    expect(addDays('2026-02-28', 1)).toBe('2026-03-01');
    expect(addDays('2026-01-01', -1)).toBe('2025-12-31');
  });

  it('разбирает дату, введённую вручную', () => {
    expect(parseTypedDate('31.12.2026')).toBe('2026-12-31');
    expect(parseTypedDate('1.2.2027')).toBe('2027-02-01');
    expect(parseTypedDate('31122026')).toBe('2026-12-31');
    expect(parseTypedDate('2026-12-31')).toBe('2026-12-31');
    expect(parseTypedDate('31.02.2026')).toBeNull();
    expect(parseTypedDate('31.12')).toBeNull();
  });

  it('расставляет точки при вводе', () => {
    expect(maskTypedDate('3')).toBe('3');
    expect(maskTypedDate('310')).toBe('31.0');
    expect(maskTypedDate('31012027')).toBe('31.01.2027');
    expect(maskTypedDate('31.01.2027 лишнее')).toBe('31.01.2027');
    expect(maskTypedDate('2027-01-31')).toBe('2027-01-31');
  });

  it('подписывает день для экранного диктора и берёт дату устройства', () => {
    expect(dayLabel('2026-10-01')).toBe('1 октября 2026, четверг');
    expect(localToday(new Date(2026, 0, 5, 23, 30))).toBe('2026-01-05');
  });
});
