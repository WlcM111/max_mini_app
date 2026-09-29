import type { DocumentCreate } from '../../api/client';
import { uuidV4 } from '../../shared/lib/uuid';
import { emptyDocumentForm, toCreateBody, validateDocumentForm, type DocumentFormState } from './documentForm';

export type ImportField = 'title' | 'number' | 'issuer' | 'validFrom' | 'validUntil' | 'responsible' | 'notes';
export type Mapping = Record<ImportField, number>; // -1 — столбец не загружается

export const IMPORT_FIELDS: { key: ImportField; label: string }[] = [
  { key: 'title', label: 'Название' },
  { key: 'number', label: 'Номер' },
  { key: 'issuer', label: 'Кем выдан' },
  { key: 'validFrom', label: 'Действует с' },
  { key: 'validUntil', label: 'Действует до' },
  { key: 'responsible', label: 'Ответственный' },
  { key: 'notes', label: 'Заметки' },
];

// Порядок важен: «Номер документа» — это номер, а не название.
const HINTS: [ImportField, RegExp][] = [
  ['number', /номер|^№|number/i],
  ['issuer', /кем выда|выдавш|орган|issuer/i],
  ['validFrom', /действует с|дата выдачи|начал|valid.?from|^с$/i],
  ['validUntil', /действует до|окончан|срок|истека|^до$|valid.?(until|to)|expir/i],
  ['responsible', /ответствен|responsible/i],
  ['notes', /замет|примечан|коммент|note/i],
  ['title', /назван|наименован|документ|title|name/i],
];

const EMPTY: Mapping = { title: -1, number: -1, issuer: -1, validFrom: -1, validUntil: -1, responsible: -1, notes: -1 };

/** Сопоставляет столбцы с полями по заголовкам; без заголовков — по порядку. */
export function detectMapping(rows: string[][]): { mapping: Mapping; hasHeader: boolean } {
  const mapping: Mapping = { ...EMPTY };
  (rows[0] ?? []).forEach((cell, index) => {
    const text = cell.trim();
    if (!text) return;
    const hit = HINTS.find(([field, pattern]) => mapping[field] === -1 && pattern.test(text));
    if (hit) mapping[hit[0]] = index;
  });
  const found = Object.values(mapping).filter((value) => value !== -1).length;
  if (mapping.title !== -1 || found >= 2) return { mapping, hasHeader: true };
  const width = rows[0]?.length ?? 0;
  const positional: ImportField[] = ['title', 'number', 'issuer', 'validFrom', 'validUntil'];
  const byPosition: Mapping = { ...EMPTY };
  positional.forEach((field, index) => {
    byPosition[field] = index < width ? index : -1;
  });
  return { mapping: byPosition, hasHeader: false };
}

export type ParsedDate = { kind: 'empty' } | { kind: 'indefinite' } | { kind: 'date'; value: string } | { kind: 'invalid'; raw: string };

const pad = (value: number) => String(value).padStart(2, '0');

function isoDate(year: number, month: number, day: number): string | null {
  const full = year < 100 ? 2000 + year : year;
  const date = new Date(Date.UTC(full, month - 1, day));
  if (date.getUTCFullYear() !== full || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return null;
  return `${full}-${pad(month)}-${pad(day)}`;
}

/** Дата из ячейки: 31.12.2026, 2026-12-31, число-дата Excel, «бессрочно». */
export function parseDateCell(raw: string): ParsedDate {
  const text = raw.trim();
  if (!text) return { kind: 'empty' };
  if (/^(бессроч|без срока|∞|—|-$)/i.test(text)) return { kind: 'indefinite' };
  let match = /^(\d{1,2})[./-](\d{1,2})[./-](\d{4}|\d{2})(?!\d)/.exec(text);
  if (match) {
    const value = isoDate(Number(match[3]), Number(match[2]), Number(match[1]));
    return value ? { kind: 'date', value } : { kind: 'invalid', raw: text };
  }
  match = /^(\d{4})-(\d{1,2})-(\d{1,2})/.exec(text);
  if (match) {
    const value = isoDate(Number(match[1]), Number(match[2]), Number(match[3]));
    return value ? { kind: 'date', value } : { kind: 'invalid', raw: text };
  }
  if (/^\d+(\.\d+)?$/.test(text)) {
    const serial = Math.floor(Number(text));
    if (serial > 20_000 && serial < 80_000) {
      const date = new Date(Date.UTC(1899, 11, 30) + serial * 86_400_000);
      return { kind: 'date', value: `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}` };
    }
  }
  return { kind: 'invalid', raw: text };
}

export interface ImportRow {
  key: string;
  line: number;
  title: string;
  validUntil: string | null;
  indefinite: boolean;
  errors: string[];
  body: DocumentCreate | null;
  /** Документ с таким названием уже есть в реестре (строка не блокируется). */
  duplicate?: boolean;
}

/** Название для сравнения: без регистра и лишних пробелов. */
export function normalizeTitle(title: string): string {
  return title.trim().replace(/\s+/g, ' ').toLowerCase();
}

/**
 * Строки файла → документы с проверкой теми же правилами, что и форма.
 * existing — нормализованные названия документов реестра (normalizeTitle).
 */
export function buildRows(
  rows: string[][],
  mapping: Mapping,
  hasHeader: boolean,
  offsets: number[] = [30, 7, 1],
  existing: ReadonlySet<string> = new Set(),
): ImportRow[] {
  const data = hasHeader ? rows.slice(1) : rows;
  const seen = new Set<string>();
  return data.map((cells, index) => {
    const get = (field: ImportField) => {
      const column = mapping[field];
      return column >= 0 ? (cells[column] ?? '').trim() : '';
    };
    const errors: string[] = [];
    const from = parseDateCell(get('validFrom'));
    const until = parseDateCell(get('validUntil'));
    if (from.kind === 'invalid') errors.push(`Дата начала «${from.raw}» не распознана`);
    if (until.kind === 'invalid') errors.push(`Дата окончания «${until.raw}» не распознана`);
    const form: DocumentFormState = {
      ...emptyDocumentForm(offsets),
      title: get('title'),
      number: get('number'),
      issuer: get('issuer'),
      responsibleLabel: get('responsible'),
      notes: get('notes'),
      validFrom: from.kind === 'date' ? from.value : null,
      validUntil: until.kind === 'date' ? until.value : null,
      indefinite: until.kind === 'indefinite',
    };
    for (const message of Object.values(validateDocumentForm(form))) if (message) errors.push(message);
    const duplicateKey = form.title.toLowerCase();
    if (form.title && seen.has(duplicateKey)) errors.push('Повторяется в файле');
    seen.add(duplicateKey);
    const line = index + (hasHeader ? 2 : 1);
    return {
      key: `line-${line}`,
      line,
      title: form.title,
      validUntil: form.validUntil,
      indefinite: form.indefinite,
      errors: [...new Set(errors)],
      body: errors.length === 0 ? toCreateBody(uuidV4(), form) : null,
      duplicate: form.title !== '' && existing.has(normalizeTitle(form.title)),
    };
  });
}
