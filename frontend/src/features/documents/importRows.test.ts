import { describe, expect, it } from 'vitest';
import { buildRows, detectMapping, parseDateCell } from './importRows';

describe('T-FE-IMPORT: разбор строк таблицы (FR-24)', () => {
  it('сопоставляет столбцы по заголовкам: «Номер документа» — номер, а не название', () => {
    const { mapping, hasHeader } = detectMapping([
      ['Номер документа', 'Наименование', 'Кем выдан', 'Дата выдачи', 'Срок действия', 'Ответственный', 'Примечание'],
    ]);
    expect(hasHeader).toBe(true);
    expect(mapping).toEqual({ number: 0, title: 1, issuer: 2, validFrom: 3, validUntil: 4, responsible: 5, notes: 6 });
  });

  it('без заголовков берёт столбцы по порядку', () => {
    const { mapping, hasHeader } = detectMapping([['Лицензия', '78РПА', 'Комитет', '14.03.2024', '13.03.2029']]);
    expect(hasHeader).toBe(false);
    expect(mapping).toMatchObject({ title: 0, number: 1, issuer: 2, validFrom: 3, validUntil: 4, responsible: -1, notes: -1 });
  });

  it.each([
    ['31.12.2026', { kind: 'date', value: '2026-12-31' }],
    ['31/12/26', { kind: 'date', value: '2026-12-31' }],
    ['2026-12-31', { kind: 'date', value: '2026-12-31' }],
    ['46387', { kind: 'date', value: '2026-12-31' }],
    ['бессрочно', { kind: 'indefinite' }],
    ['  ', { kind: 'empty' }],
    ['31.02.2026', { kind: 'invalid', raw: '31.02.2026' }],
    ['завтра', { kind: 'invalid', raw: 'завтра' }],
  ])('дата в ячейке «%s»', (raw, expected) => {
    expect(parseDateCell(raw)).toEqual(expected);
  });

  it('проверяет строки правилами формы и собирает тело запроса', () => {
    const table = [
      ['Название', 'Действует с', 'Действует до'],
      ['Лицензия на алкоголь', '14.03.2024', '13.03.2029'],
      ['', '', '01.01.2027'],
      ['Договор аренды', '01.02.2026', '31.01.2026'],
      ['лицензия на алкоголь', '', '13.03.2029'],
      ['Устав', '', 'бессрочно'],
      ['Сертификат', '', '32.13.2026'],
    ];
    const { mapping, hasHeader } = detectMapping(table);
    const rows = buildRows(table, mapping, hasHeader);
    expect(rows.map((row) => row.line)).toEqual([2, 3, 4, 5, 6, 7]);
    expect(rows[0]?.body).toMatchObject({
      title: 'Лицензия на алкоголь',
      valid_from: '2024-03-14',
      valid_until: '2029-03-13',
      reminder_offsets_days: [30, 7, 1],
    });
    expect(rows[1]?.errors).toContain('Введите название (до 200 символов)');
    expect(rows[2]?.errors).toContain('Дата окончания раньше даты начала');
    expect(rows[3]?.errors).toContain('Повторяется в файле');
    expect(rows[4]).toMatchObject({ indefinite: true, validUntil: null });
    expect(rows[4]?.body?.valid_until).toBeNull();
    expect(rows[5]?.errors).toContain('Дата окончания «32.13.2026» не распознана');
    expect(rows.filter((row) => row.body !== null)).toHaveLength(2);
  });

  it('идентификаторы строк стабильны при повторном разборе с теми же настройками', () => {
    const table = [['Название'], ['Устав']];
    const { mapping, hasHeader } = detectMapping(table);
    const [first] = buildRows(table, mapping, hasHeader);
    expect(first?.key).toBe('line-2');
    expect(first?.body?.id).toMatch(/^[0-9a-f-]{36}$/);
  });
});
