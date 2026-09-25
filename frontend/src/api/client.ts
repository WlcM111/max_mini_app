import createClient from 'openapi-fetch';
import type { components, paths } from './schema';
import { ApiError, NetworkError } from './errors';
import { clearSession, getLaunchContext, getToken, setSession } from '../session/sessionStore';

// Единственное место проекта, где выполняются сетевые запросы к публичному API.
// Типы запросов и ответов берутся только из сгенерированной схемы OpenAPI.

export type Account = components['schemas']['Account'];
export type Me = components['schemas']['Me'];
export type Catalog = components['schemas']['Catalog'];
export type DocumentType = components['schemas']['DocumentType'];
export type Organization = components['schemas']['Organization'];
export type DocumentStats = components['schemas']['DocumentStats'];
export type DocumentListItem = components['schemas']['DocumentListItem'];
export type DocumentPage = components['schemas']['DocumentPage'];
export type Document = components['schemas']['Document'];
export type Period = components['schemas']['Period'];
export type Member = components['schemas']['Member'];
export type InviteCreated = components['schemas']['InviteCreated'];
export type InviteSummary = components['schemas']['InviteSummary'];
export type InvitePreview = components['schemas']['InvitePreview'];
export type NotificationSettings = components['schemas']['NotificationSettings'];
export type CalendarExport = components['schemas']['CalendarExport'];
export type Suggestion = components['schemas']['Suggestion'];
export type Session = components['schemas']['Session'];
export type Role = components['schemas']['Role'];
export type DeadlineStatus = components['schemas']['DeadlineStatus'];
export type RemindersState = components['schemas']['RemindersState'];
export type DocumentCreate = components['schemas']['DocumentCreate'];
export type DocumentUpdate = components['schemas']['DocumentUpdate'];
export type OrganizationCreate = components['schemas']['OrganizationCreate'];
export type OrganizationUpdate = components['schemas']['OrganizationUpdate'];
export type Limits = components['schemas']['Limits'];
export type MembershipSummary = components['schemas']['MembershipSummary'];
export type RemindersChannel = components['schemas']['RemindersChannel'];
export type Feature = components['schemas']['Feature'];
export type Region = components['schemas']['Region'];
export type BusinessCategory = components['schemas']['BusinessCategory'];
export type InviteAccepted = components['schemas']['InviteAccepted'];
export type RenewalCreate = components['schemas']['RenewalCreate'];

const DEFAULT_TIMEOUT_MS = 10_000;

export const apiBaseUrl = (): string => import.meta.env.VITE_API_BASE_URL ?? '/api/v1';

// fetch связывается поздно (в момент вызова): так клиент работает и в браузере,
// и в тестовой среде, где сетевой слой подменяется после загрузки модулей.
export const client = createClient<paths>({
  baseUrl: apiBaseUrl(),
  fetch: (request) => globalThis.fetch(request),
});

interface Result<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

/**
 * Выполняет запрос: добавляет заголовок авторизации, ограничивает время ожидания,
 * один раз обновляет сессию при 401 и превращает problem+json в ApiError.
 */
export async function run<T>(
  exec: (init: { headers: Record<string, string>; signal: AbortSignal }) => Promise<Result<T>>,
  options: { retryOn401?: boolean; timeoutMs?: number } = {},
): Promise<T> {
  const attempt = async (): Promise<Result<T>> => {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), options.timeoutMs ?? DEFAULT_TIMEOUT_MS);
    try {
      return await exec({ headers: authHeaders(), signal: controller.signal });
    } catch (reason) {
      throw new NetworkError(reason);
    } finally {
      clearTimeout(timer);
    }
  };

  let result = await attempt();
  if (result.response.status === 401 && options.retryOn401 !== false) {
    const refreshed = await refreshSession();
    if (refreshed) result = await attempt();
  }
  if (!result.response.ok) throw ApiError.fromResponse(result.response, result.error);
  return result.data as T;
}

function authHeaders(): Record<string, string> {
  const token = getToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

/** Создаёт сессию по данным запуска MAX. */
export async function createSession(initData: string, platform: string, appVersion: string): Promise<Session> {
  const body: components['schemas']['SessionCreateRequest'] = {
    init_data: initData,
    platform: platform as components['schemas']['SessionCreateRequest']['platform'],
    app_version: appVersion,
  };
  const session = await run<Session>(
    ({ signal }) => client.POST('/sessions', { body, signal }),
    { retryOn401: false },
  );
  setSession({
    token: session.token,
    expiresAt: session.expires_at,
    account: session.account,
    start: session.start,
  });
  return session;
}

let refreshing: Promise<boolean> | null = null;

/** Однократное обновление сессии по сохранённой строке запуска (архитектура §5). */
export function refreshSession(): Promise<boolean> {
  if (!refreshing) {
    refreshing = doRefresh().finally(() => {
      refreshing = null;
    });
  }
  return refreshing;
}

async function doRefresh(): Promise<boolean> {
  const launch = getLaunchContext();
  if (!launch || launch.initData.length === 0) {
    clearSession();
    return false;
  }
  try {
    await createSession(launch.initData, launch.platform, launch.appVersion);
    return true;
  } catch {
    clearSession();
    return false;
  }
}

/** Завершает текущую сессию на сервере и очищает клиентское состояние. */
export async function deleteCurrentSession(): Promise<void> {
  try {
    await run(({ headers, signal }) => client.DELETE('/sessions/current', { headers, signal }), { retryOn401: false });
  } finally {
    clearSession();
  }
}
