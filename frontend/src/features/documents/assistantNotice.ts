import type { DocumentDraft } from '../../api/client';

export interface AssistNotice {
  tone: 'info' | 'warning';
  text: string;
}

/** Итог распознавания: какие поля заполнены и чего не хватает для напоминаний. */
export function draftNotice(draft: DocumentDraft, source: 'text' | 'photo'): AssistNotice {
  const found = [
    draft.title ? 'название' : null,
    draft.number ? 'номер' : null,
    draft.issuer ? 'кем выдан' : null,
    draft.valid_from ? 'дата начала' : null,
    draft.valid_until ? 'дата окончания' : null,
  ].filter((item): item is string => item !== null);
  const by = source === 'text' ? 'по тексту' : 'по фото';
  const where = source === 'text' ? 'в тексте' : 'на фото';
  if (found.length === 0) return { tone: 'warning', text: 'Реквизиты не найдены — заполните поля вручную.' };
  const head = `Заполнено ${by}: ${found.join(', ')}.`;
  if (!draft.valid_until) {
    return {
      tone: 'warning',
      text: `${head} Дата окончания ${where} не найдена — выберите её в поле «Действует до» или отметьте документ бессрочным.`,
    };
  }
  if (draft.confidence < 0.5) return { tone: 'warning', text: `${head} Распознано неуверенно — сверьте поля с документом.` };
  return { tone: 'info', text: `${head} Проверьте поля перед сохранением.` };
}
