import { beforeEach, describe, expect, it } from 'vitest';
import {
  checklistKey,
  clearChecklist,
  normalizeChecklist,
  progressText,
  readChecklist,
  saveChecklist,
  toggleStep,
} from './renewalChecklist';

const DOC = '2f8b6d41-9c3e-4a75-b1d8-6e5f4a3c2b10';

describe('T-FE-VAL: чек-лист продления', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('хранит отметки по ключу документа', () => {
    expect(checklistKey(DOC)).toBe(`vovremya.renewal.${DOC}`);
    saveChecklist(DOC, [0, 2]);
    expect(window.localStorage.getItem(checklistKey(DOC))).toBe('[0,2]');
    expect(readChecklist(DOC, 4)).toEqual([0, 2]);
  });

  it('пустой список удаляет запись, а не хранит её', () => {
    saveChecklist(DOC, [1]);
    saveChecklist(DOC, []);
    expect(window.localStorage.getItem(checklistKey(DOC))).toBeNull();
    expect(readChecklist(DOC, 4)).toEqual([]);
  });

  it('переключает шаг в обе стороны и держит порядок', () => {
    expect(toggleStep([], 2)).toEqual([2]);
    expect(toggleStep([2], 0)).toEqual([0, 2]);
    expect(toggleStep([0, 2], 2)).toEqual([0]);
  });

  it('отбрасывает мусор: повторы, чужие типы и шаги вне диапазона', () => {
    expect(normalizeChecklist([1, 1, 2], 3)).toEqual([1, 2]);
    expect(normalizeChecklist([-1, 0, 5], 3)).toEqual([0]);
    expect(normalizeChecklist(['1', null, 1.5, 2], 3)).toEqual([2]);
    expect(normalizeChecklist('не массив', 3)).toEqual([]);
  });

  it('сокращение списка шагов не ломает чтение', () => {
    saveChecklist(DOC, [0, 3]);
    expect(readChecklist(DOC, 2)).toEqual([0]);
  });

  it('повреждённое значение считается пустым и удаляется', () => {
    window.localStorage.setItem(checklistKey(DOC), '{не json');
    expect(readChecklist(DOC, 3)).toEqual([]);
    expect(window.localStorage.getItem(checklistKey(DOC))).toBeNull();
  });

  it('снятие всех отметок очищает запись', () => {
    saveChecklist(DOC, [0, 1]);
    clearChecklist(DOC);
    expect(readChecklist(DOC, 2)).toEqual([]);
  });

  it('текст прогресса зависит от числа выполненных шагов', () => {
    expect(progressText(0, 0)).toBe('Шагов нет');
    expect(progressText(0, 4)).toBe('Шагов: 4');
    expect(progressText(1, 4)).toBe('Готово 1 из 4');
    expect(progressText(4, 4)).toBe('Всё готово');
  });
});
