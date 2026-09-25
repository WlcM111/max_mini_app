import { describe, expect, it } from 'vitest';
import {
  notifyTimeOptions,
  validateDates,
  validateDocumentTitle,
  validateNotifyTime,
  validateOffsets,
  validateOrganizationName,
  validateReferenceUrl,
} from './validation';
import { isUuidV4, uuidV4 } from './uuid';

describe('T-FE-VAL: правила форм на границах', () => {
  it('проверяет название организации', () => {
    expect(validateOrganizationName('Кафе')).toBeNull();
    expect(validateOrganizationName('   ')).not.toBeNull();
    expect(validateOrganizationName('a'.repeat(100))).toBeNull();
    expect(validateOrganizationName('a'.repeat(101))).not.toBeNull();
  });

  it('проверяет название документа', () => {
    expect(validateDocumentTitle('Лицензия')).toBeNull();
    expect(validateDocumentTitle('')).not.toBeNull();
    expect(validateDocumentTitle('a'.repeat(200))).toBeNull();
    expect(validateDocumentTitle('a'.repeat(201))).not.toBeNull();
  });

  it('требует https в ссылке на источник', () => {
    expect(validateReferenceUrl('')).toBeNull();
    expect(validateReferenceUrl('https://example.test/doc')).toBeNull();
    expect(validateReferenceUrl('http://example.test')).not.toBeNull();
    expect(validateReferenceUrl(`https://example.test/${'a'.repeat(1024)}`)).not.toBeNull();
  });

  it('сверяет даты периода', () => {
    expect(validateDates('2026-01-01', '2026-01-01')).toBeNull();
    expect(validateDates('2026-02-01', '2026-01-01')).toBe('Дата окончания раньше даты начала');
    expect(validateDates(null, null)).toBeNull();
    expect(validateDates('2026-13-01', null)).not.toBeNull();
  });

  it('ограничивает отступы напоминаний', () => {
    expect(validateOffsets([30, 7, 1])).toBeNull();
    expect(validateOffsets([])).toBeNull();
    expect(validateOffsets([90, 60, 30, 14, 7])).toBeNull();
    expect(validateOffsets([90, 60, 30, 14, 7, 3])).toBe('Не более 5 напоминаний');
    expect(validateOffsets([7, 7])).toBe('Значения не должны повторяться');
    expect(validateOffsets([366])).not.toBeNull();
    expect(validateOffsets([-1])).not.toBeNull();
    expect(validateOffsets([0])).toBeNull();
  });

  it('ограничивает время напоминаний шагом 30 минут в 06:00–22:00', () => {
    expect(validateNotifyTime('09:00')).toBeNull();
    expect(validateNotifyTime('06:00')).toBeNull();
    expect(validateNotifyTime('22:00')).toBeNull();
    expect(validateNotifyTime('22:30')).not.toBeNull();
    expect(validateNotifyTime('05:30')).not.toBeNull();
    expect(validateNotifyTime('09:15')).not.toBeNull();
    expect(validateNotifyTime('девять')).not.toBeNull();
    expect(notifyTimeOptions()).toHaveLength(33);
  });

  it('генерирует UUID версии 4 по шаблону OpenAPI', () => {
    const value = uuidV4();
    expect(isUuidV4(value)).toBe(true);
    expect(isUuidV4('0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01')).toBe(true);
    expect(isUuidV4('не-uuid')).toBe(false);
    expect(uuidV4()).not.toBe(value);
  });
});
