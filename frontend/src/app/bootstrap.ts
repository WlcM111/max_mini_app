import { ApiError, NetworkError } from '../api/errors';
import { createSession, type Catalog, type Me, type Session } from '../api/client';
import { getBridge } from '../platform/max/bridge';
import type { MaxBridge } from '../platform/max/types';
import { getCatalog, getMe } from '../features/organizations/api';
import {
  getLastOrganization,
  getSession,
  restoreSession,
  setLaunchContext,
  type SessionState,
  type StartTarget,
} from '../session/sessionStore';
import { track } from '../shared/lib/telemetry';

export interface BootstrapReady {
  status: 'ready';
  bridge: MaxBridge;
  session: SessionState;
  me: Me;
  catalog: Catalog;
  initialPath: string;
  inviteToken: string | null;
}

export type BootstrapOutcome =
  | BootstrapReady
  | { status: 'not-in-max' }
  | { status: 'launch-error'; message: string }
  | { status: 'network-error'; message: string; retryAfterSeconds?: number };

const appVersion = (): string => import.meta.env.VITE_APP_VERSION ?? 'dev';

/** Выбирает первый экран по цели запуска и списку организаций (spec §3, шаг 5). */
export function chooseInitialPath(start: StartTarget, me: Me, lastOrganization: string | null): string {
  if (start.kind === 'document' && start.document_id) return `/d/${start.document_id}`;
  if (start.kind === 'renew' && start.document_id) return `/d/${start.document_id}/renew`;
  if (start.kind === 'organization' && start.organization_id) return `/o/${start.organization_id}`;
  if (start.kind === 'invite') return '/invite';
  if (me.memberships.length === 0) return '/welcome';
  const known = me.memberships.find((item) => item.organization_id === lastOrganization);
  const target = known ?? me.memberships[0];
  return target ? `/o/${target.organization_id}` : '/welcome';
}

/**
 * Полный запуск мини-приложения: Bridge → сессия → профиль и справочник → первый экран.
 * Ошибки приводятся к экранам «Откройте из MAX», «Ошибка запуска» и «Нет соединения».
 */
export async function bootstrap(): Promise<BootstrapOutcome> {
  const started = performance.now();
  const bridge = await getBridge();
  if (bridge.kind === 'unavailable' || bridge.initData.length === 0) {
    return { status: 'not-in-max' };
  }
  setLaunchContext({ initData: bridge.initData, platform: bridge.platform, appVersion: appVersion() });

  try {
    const restored = restoreSession();
    let start: StartTarget = { kind: 'none' };
    if (!restored) {
      const session: Session = await createSession(bridge.initData, bridge.platform, appVersion());
      start = session.start;
    }
    const session = getSession();
    if (!session) return { status: 'launch-error', message: 'Не удалось подтвердить запуск из MAX' };

    const [me, catalog] = await Promise.all([getMe(), getCatalog()]);
    const initialPath = chooseInitialPath(start, me, getLastOrganization());
    track({
      name: 'bootstrap_completed',
      durationMs: performance.now() - started,
      platform: bridge.platform,
    });
    return {
      status: 'ready',
      bridge,
      session,
      me,
      catalog,
      initialPath,
      inviteToken: start.kind === 'invite' ? (start.invite_token ?? null) : null,
    };
  } catch (error) {
    if (error instanceof NetworkError) {
      track({ name: 'bootstrap_failed', code: 'network' });
      return { status: 'network-error', message: 'Нет соединения. Проверьте сеть и повторите.' };
    }
    if (error instanceof ApiError) {
      if (error.code === 'LAUNCH_DATA_EXPIRED') {
        track({ name: 'bootstrap_failed', code: 'launch_expired' });
        return { status: 'launch-error', message: 'Сессия запуска устарела — закройте и снова откройте приложение' };
      }
      if (error.code === 'LAUNCH_DATA_INVALID' || error.status === 401) {
        track({ name: 'bootstrap_failed', code: 'launch_invalid' });
        return { status: 'launch-error', message: 'Не удалось подтвердить запуск из MAX' };
      }
      if (error.status === 429 || error.status === 503) {
        track({ name: 'bootstrap_failed', code: 'server_5xx' });
        return {
          status: 'network-error',
          message: 'Сервис занят. Повторите попытку.',
          ...(error.retryAfterSeconds !== undefined ? { retryAfterSeconds: error.retryAfterSeconds } : {}),
        };
      }
      track({ name: 'bootstrap_failed', code: 'server_5xx' });
      return { status: 'network-error', message: 'Сервис временно недоступен. Повторите попытку.' };
    }
    track({ name: 'bootstrap_failed', code: 'server_5xx' });
    return { status: 'network-error', message: 'Не удалось запустить приложение. Повторите попытку.' };
  }
}
