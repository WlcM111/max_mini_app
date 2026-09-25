import { readStorage, removeStorage, writeStorage } from '../shared/lib/storage';
import type { components } from '../api/schema';

export type Account = components['schemas']['Account'];
export type StartTarget = components['schemas']['StartTarget'];

export interface SessionState {
  token: string;
  expiresAt: string;
  account: Account;
  start: StartTarget;
}

export interface LaunchContext {
  initData: string;
  platform: string;
  appVersion: string;
}

const SESSION_KEY = 'vovremya.session';
const LAST_ORG_KEY = 'vovremya.lastOrg';

let state: SessionState | null = null;
let launch: LaunchContext | null = null;
const listeners = new Set<(value: SessionState | null) => void>();

function notify(): void {
  for (const listener of listeners) listener(state);
}

/** Подписка на изменения сессии (используется React-контекстом). */
export function subscribeSession(listener: (value: SessionState | null) => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getSession(): SessionState | null {
  return state;
}

export function getToken(): string | null {
  if (!state) return null;
  return Date.parse(state.expiresAt) > Date.now() ? state.token : null;
}

export function setSession(value: SessionState): void {
  state = value;
  writeStorage('session', SESSION_KEY, JSON.stringify(value));
  notify();
}

export function clearSession(): void {
  state = null;
  removeStorage('session', SESSION_KEY);
  notify();
}

/** Восстанавливает неистёкшую сессию после перезагрузки страницы. */
export function restoreSession(): SessionState | null {
  const raw = readStorage('session', SESSION_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as SessionState;
    if (!parsed.token || !parsed.expiresAt || Date.parse(parsed.expiresAt) <= Date.now()) {
      removeStorage('session', SESSION_KEY);
      return null;
    }
    state = parsed;
    notify();
    return parsed;
  } catch {
    removeStorage('session', SESSION_KEY);
    return null;
  }
}

/** Контекст запуска нужен для повторной авторизации после 401. */
export function setLaunchContext(value: LaunchContext): void {
  launch = value;
}

export function getLaunchContext(): LaunchContext | null {
  return launch;
}

export function getLastOrganization(): string | null {
  return readStorage('local', LAST_ORG_KEY);
}

export function setLastOrganization(organizationId: string): void {
  writeStorage('local', LAST_ORG_KEY, organizationId);
}

export function clearLastOrganization(): void {
  removeStorage('local', LAST_ORG_KEY);
}
