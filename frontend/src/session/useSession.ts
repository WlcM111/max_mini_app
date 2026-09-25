import { createContext, useContext } from 'react';
import type { Me } from '../api/client';
import type { SessionState } from './sessionStore';

export interface SessionContextValue {
  session: SessionState;
  me: Me;
  platform: string;
  bridgeKind: 'max' | 'mock' | 'unavailable';
  refreshMe: () => Promise<void>;
}

export const SessionContext = createContext<SessionContextValue | null>(null);

/** Доступ к подтверждённой сессии; вне провайдера приложение не работает. */
export function useSession(): SessionContextValue {
  const value = useContext(SessionContext);
  if (!value) throw new Error('SessionContext недоступен: приложение не прошло bootstrap');
  return value;
}

/** Роль текущего пользователя в организации по данным GET /me. */
export function useRole(organizationId: string): 'owner' | 'editor' | 'viewer' | null {
  const { me } = useSession();
  const membership = me.memberships.find((item) => item.organization_id === organizationId);
  return membership ? membership.role : null;
}

/** Проверка минимальной роли: viewer < editor < owner. */
export function roleAllows(role: string | null, minimal: 'viewer' | 'editor' | 'owner'): boolean {
  const rank = { viewer: 1, editor: 2, owner: 3 } as const;
  if (!role || !(role in rank)) return false;
  return rank[role as keyof typeof rank] >= rank[minimal];
}
