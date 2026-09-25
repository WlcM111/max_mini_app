import { describe, expect, it } from 'vitest';
import {
  emptyDocumentForm,
  formFromDocument,
  isDirty,
  toCreateBody,
  toUpdateBody,
  validateDocumentForm,
} from './documentForm';
import { document } from '../../test/fixtures';

describe('T-FE-VAL: форма документа', () => {
  it('переносит карточку документа в форму', () => {
    const form = formFromDocument(document);
    expect(form.title).toBe('Лицензия на алкоголь');
    expect(form.validUntil).toBe('2026-10-01');
    expect(form.indefinite).toBe(false);
    expect(form.offsets).toEqual([60, 30, 7]);
  });

  it('собирает ошибки всех полей', () => {
    const errors = validateDocumentForm({
      ...emptyDocumentForm(),
      title: '',
      number: 'x'.repeat(101),
      referenceUrl: 'http://insecure',
      validFrom: '2026-05-01',
      validUntil: '2026-01-01',
      offsets: [1, 1],
    });
    expect(errors.title).toBeDefined();
    expect(errors.number).toBeDefined();
    expect(errors.referenceUrl).toBeDefined();
    expect(errors.validUntil).toBeDefined();
    expect(errors.offsets).toBeDefined();
  });

  it('формирует тело создания с клиентским идентификатором', () => {
    const body = toCreateBody('2f8b6d41-9c3e-4a75-b1d8-6e5f4a3c2b10', {
      ...emptyDocumentForm([30]),
      title: '  Лицензия  ',
      number: '',
      validUntil: '2026-10-01',
    });
    expect(body.id).toBe('2f8b6d41-9c3e-4a75-b1d8-6e5f4a3c2b10');
    expect(body.title).toBe('Лицензия');
    expect(body.number).toBeNull();
    expect(body.valid_until).toBe('2026-10-01');
    expect(body.reminder_offsets_days).toEqual([30]);
  });

  it('передаёт ожидаемую версию и очищает срок для бессрочного документа', () => {
    const body = toUpdateBody(4, { ...emptyDocumentForm(), title: 'Договор', indefinite: true, validUntil: '2026-10-01' });
    expect(body.expected_version).toBe(4);
    expect(body.valid_until).toBeNull();
  });

  it('определяет изменение формы для подтверждения закрытия', () => {
    const initial = emptyDocumentForm();
    expect(isDirty(initial, { ...initial })).toBe(false);
    expect(isDirty(initial, { ...initial, title: 'Новое' })).toBe(true);
  });
});
