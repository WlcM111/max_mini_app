import { readStorage, removeStorage, writeStorage } from '../../shared/lib/storage';
import { validateDocumentTitle } from '../../shared/lib/validation';
import { getDraft, updateDraft, type CustomDocument } from './onboardingDraft';

/** Выбор документов для пакетного добавления: типовые по коду и свои по названию. */
export interface DocumentPick {
  selectedTypes: string[];
  customDocuments: CustomDocument[];
}

/** Онбординг хранит выбор в своём черновике, раздел «Документы» — отдельно для каждой организации. */
export type PickScope = { kind: 'onboarding' } | { kind: 'organization'; organizationId: string };

const key = (organizationId: string) => `vovremya.pick.${organizationId}`;

export const emptyPick = (): DocumentPick => ({ selectedTypes: [], customDocuments: [] });

const isCustom = (value: unknown): value is CustomDocument =>
  typeof value === 'object' && value !== null && typeof (value as CustomDocument).id === 'string' && typeof (value as CustomDocument).title === 'string';

/** Читает выбор; повреждённое значение хранилища отбрасывается. */
export function readPick(scope: PickScope): DocumentPick {
  if (scope.kind === 'onboarding') {
    const draft = getDraft();
    return { selectedTypes: [...draft.selectedTypes], customDocuments: [...draft.customDocuments] };
  }
  const raw = readStorage('session', key(scope.organizationId));
  if (!raw) return emptyPick();
  try {
    const parsed = JSON.parse(raw) as Partial<Record<keyof DocumentPick, unknown>>;
    return {
      selectedTypes: Array.isArray(parsed.selectedTypes) ? parsed.selectedTypes.filter((code): code is string => typeof code === 'string') : [],
      customDocuments: Array.isArray(parsed.customDocuments) ? parsed.customDocuments.filter(isCustom) : [],
    };
  } catch {
    removeStorage('session', key(scope.organizationId));
    return emptyPick();
  }
}

export function writePick(scope: PickScope, pick: DocumentPick): void {
  if (scope.kind === 'onboarding') updateDraft({ selectedTypes: pick.selectedTypes, customDocuments: pick.customDocuments });
  else writeStorage('session', key(scope.organizationId), JSON.stringify(pick));
}

export function clearPick(scope: PickScope): void {
  if (scope.kind === 'onboarding') updateDraft({ selectedTypes: [], customDocuments: [] });
  else removeStorage('session', key(scope.organizationId));
}

/** Название своего документа: обязательно, до 200 символов, без повторов среди выбранных. */
export function validateCustomTitle(title: string, taken: readonly string[]): string | null {
  const invalid = validateDocumentTitle(title);
  if (invalid) return invalid;
  const normalized = title.trim().toLowerCase();
  return taken.some((item) => item.trim().toLowerCase() === normalized) ? 'Такой документ уже в списке' : null;
}
