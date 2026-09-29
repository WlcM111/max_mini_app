import { readStorage, removeStorage, writeStorage } from '../../shared/lib/storage';
import { uuidV4 } from '../../shared/lib/uuid';

const KEY = 'vovremya.onboarding';

/** Свой документ, которого нет среди типовых: название задаёт пользователь, срок — на следующем шаге. */
export interface CustomDocument {
  id: string;
  title: string;
}

export interface OnboardingDraft {
  organizationId: string;
  name: string;
  businessCategoryCode: string;
  regionCode: string;
  timezone: string;
  featureCodes: string[];
  selectedTypes: string[];
  customDocuments: CustomDocument[];
}

const empty = (): OnboardingDraft => ({
  organizationId: uuidV4(),
  name: '',
  businessCategoryCode: '',
  regionCode: '',
  timezone: '',
  featureCodes: [],
  selectedTypes: [],
  customDocuments: [],
});

let draft: OnboardingDraft | null = null;

/** Черновик онбординга живёт в памяти и переживает перезагрузку страницы. */
export function getDraft(): OnboardingDraft {
  if (draft) return draft;
  const raw = readStorage('session', KEY);
  if (raw) {
    try {
      // Черновик прежней версии мог не содержать своих документов.
      draft = { ...empty(), ...(JSON.parse(raw) as Partial<OnboardingDraft>) };
      return draft;
    } catch {
      removeStorage('session', KEY);
    }
  }
  draft = empty();
  return draft;
}

export function updateDraft(patch: Partial<OnboardingDraft>): OnboardingDraft {
  draft = { ...getDraft(), ...patch };
  writeStorage('session', KEY, JSON.stringify(draft));
  return draft;
}

export function resetDraft(): void {
  draft = null;
  removeStorage('session', KEY);
}
