import type { Document, DocumentCreate, DocumentUpdate } from '../../api/client';
import {
  validateDates,
  validateDocumentTitle,
  validateMaxLength,
  validateOffsets,
  validateReferenceUrl,
} from '../../shared/lib/validation';

export interface DocumentFormState {
  documentTypeCode: string | null;
  title: string;
  number: string;
  issuer: string;
  responsibleLabel: string;
  notes: string;
  referenceUrl: string;
  validFrom: string | null;
  validUntil: string | null;
  indefinite: boolean;
  offsets: number[];
}

export type FormErrors = Partial<Record<keyof DocumentFormState | 'form', string>>;

export const emptyDocumentForm = (offsets: number[] = [30, 7, 1]): DocumentFormState => ({
  documentTypeCode: null,
  title: '',
  number: '',
  issuer: '',
  responsibleLabel: '',
  notes: '',
  referenceUrl: '',
  validFrom: null,
  validUntil: null,
  indefinite: false,
  offsets,
});

/** Заполняет форму по карточке документа для режима изменения. */
export function formFromDocument(document: Document): DocumentFormState {
  return {
    documentTypeCode: document.document_type_code ?? null,
    title: document.title,
    number: document.number ?? '',
    issuer: document.issuer ?? '',
    responsibleLabel: document.responsible_label ?? '',
    notes: document.notes ?? '',
    referenceUrl: document.reference_url ?? '',
    validFrom: document.current_period.valid_from ?? null,
    validUntil: document.current_period.valid_until ?? null,
    indefinite: document.current_period.valid_until === null,
    offsets: [...document.reminder_offsets_days],
  };
}

/** Клиентская проверка формы: те же границы, что в OpenAPI (§8 архитектуры). */
export function validateDocumentForm(form: DocumentFormState): FormErrors {
  const errors: FormErrors = {};
  const title = validateDocumentTitle(form.title);
  if (title) errors.title = title;
  const number = validateMaxLength(form.number, 100, 'Не длиннее 100 символов');
  if (number) errors.number = number;
  const issuer = validateMaxLength(form.issuer, 200, 'Не длиннее 200 символов');
  if (issuer) errors.issuer = issuer;
  const responsible = validateMaxLength(form.responsibleLabel, 100, 'Не длиннее 100 символов');
  if (responsible) errors.responsibleLabel = responsible;
  const notes = validateMaxLength(form.notes, 2000, 'Не длиннее 2000 символов');
  if (notes) errors.notes = notes;
  const reference = validateReferenceUrl(form.referenceUrl);
  if (reference) errors.referenceUrl = reference;
  const dates = validateDates(form.validFrom, form.indefinite ? null : form.validUntil);
  if (dates) errors.validUntil = dates;
  const offsets = validateOffsets(form.offsets);
  if (offsets) errors.offsets = offsets;
  return errors;
}

const trimmed = (value: string): string | null => {
  const result = value.trim();
  return result.length === 0 ? null : result;
};

/** Тело запроса создания документа; идентификатор задаёт клиент (идемпотентность). */
export function toCreateBody(id: string, form: DocumentFormState): DocumentCreate {
  return {
    id,
    document_type_code: form.documentTypeCode,
    title: form.title.trim(),
    number: trimmed(form.number),
    issuer: trimmed(form.issuer),
    responsible_label: trimmed(form.responsibleLabel),
    notes: trimmed(form.notes),
    reference_url: trimmed(form.referenceUrl),
    valid_from: form.validFrom,
    valid_until: form.indefinite ? null : form.validUntil,
    reminder_offsets_days: form.offsets,
  };
}

/** Тело запроса изменения: передаются все редактируемые поля и ожидаемая версия. */
export function toUpdateBody(expectedVersion: number, form: DocumentFormState): DocumentUpdate {
  // Тип документа в контракте изменения не передаётся (OpenAPI DocumentUpdate).
  return {
    expected_version: expectedVersion,
    title: form.title.trim(),
    number: trimmed(form.number),
    issuer: trimmed(form.issuer),
    responsible_label: trimmed(form.responsibleLabel),
    notes: trimmed(form.notes),
    reference_url: trimmed(form.referenceUrl),
    valid_from: form.validFrom,
    valid_until: form.indefinite ? null : form.validUntil,
    reminder_offsets_days: form.offsets,
  };
}

/** Проверяет, изменилась ли форма (для подтверждения закрытия приложения). */
export function isDirty(initial: DocumentFormState, current: DocumentFormState): boolean {
  return JSON.stringify(initial) !== JSON.stringify(current);
}
