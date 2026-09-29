import { describe, expect, it } from 'vitest';
import type { DocumentDraft } from '../../api/client';
import { draftNotice } from './assistantNotice';

const base: DocumentDraft = {
  title: 'Лицензия на алкоголь',
  number: null,
  issuer: null,
  valid_from: null,
  valid_until: null,
  document_type_code: null,
  reminder_offsets_days: [],
  confidence: 0.6,
};

describe('T-FE-LLM: итог распознавания', () => {
  it('называет заполненные поля и просит проверить их', () => {
    expect(draftNotice({ ...base, number: '78РПА', valid_until: '2029-03-13', confidence: 0.9 }, 'photo')).toEqual({
      tone: 'info',
      text: 'Заполнено по фото: название, номер, дата окончания. Проверьте поля перед сохранением.',
    });
  });

  it('предупреждает, что даты окончания в тексте нет', () => {
    const notice = draftNotice({ ...base, valid_from: '2022-03-12' }, 'text');
    expect(notice.tone).toBe('warning');
    expect(notice.text).toBe(
      'Заполнено по тексту: название, дата начала. Дата окончания в тексте не найдена — выберите её в поле «Действует до» или отметьте документ бессрочным.',
    );
  });

  it('при низкой уверенности просит сверить поля с документом', () => {
    const notice = draftNotice({ ...base, valid_until: '2029-03-13', confidence: 0.3 }, 'text');
    expect(notice).toEqual({ tone: 'warning', text: 'Заполнено по тексту: название, дата окончания. Распознано неуверенно — сверьте поля с документом.' });
  });
});
