import { readStorage, removeStorage, writeStorage } from '../../shared/lib/storage';
import { uuidV4 } from '../../shared/lib/uuid';

const KEY = 'vovremya.onboarding';

export interface OnboardingDraft {
  organizationId: string;
  name: string;
  businessCategoryCode: string;
  regionCode: string;
  timezone: string;
  featureCodes: string[];
  selectedTypes: string[];
}

const empty = (): OnboardingDraft => ({
  organizationId: uuidV4(),
  name: '',
  businessCategoryCode: '',
  regionCode: '',
  timezone: '',
  featureCodes: [],
  selectedTypes: [],
});

let draft: OnboardingDraft | null = null;

/** Черновик онбординга живёт в памяти и переживает перезагрузку страницы. */
export function getDraft(): OnboardingDraft {
  if (draft) return draft;
  const raw = readStorage('session', KEY);
  if (raw) {
    try {
      draft = JSON.parse(raw) as OnboardingDraft;
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
