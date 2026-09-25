import type { components } from './schema';

export type Problem = components['schemas']['Problem'];
export type ErrorCode = components['schemas']['ErrorCode'];

/** Ошибка публичного API: разобранный ответ application/problem+json. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly detail: string | undefined;
  readonly fields: { field: string; code: string; message: string }[];
  readonly requestId: string | undefined;
  readonly retryAfterSeconds: number | undefined;

  constructor(init: {
    status: number;
    code: string;
    title?: string;
    detail?: string;
    fields?: { field: string; code: string; message: string }[];
    requestId?: string;
    retryAfterSeconds?: number;
  }) {
    super(init.title ?? init.detail ?? `Ошибка запроса (${init.status})`);
    this.name = 'ApiError';
    this.status = init.status;
    this.code = init.code;
    this.detail = init.detail;
    this.fields = init.fields ?? [];
    this.requestId = init.requestId;
    this.retryAfterSeconds = init.retryAfterSeconds;
  }

  /** Ошибки конкретных полей формы: field → сообщение. */
  fieldErrors(): Record<string, string> {
    const result: Record<string, string> = {};
    for (const item of this.fields) result[item.field] = item.message;
    return result;
  }

  static fromResponse(response: Response, payload: unknown): ApiError {
    const problem = (payload ?? {}) as Partial<Problem>;
    const retryAfter = Number(response.headers.get('Retry-After') ?? '');
    return new ApiError({
      status: response.status,
      code: typeof problem.code === 'string' ? problem.code : httpFallbackCode(response.status),
      title: typeof problem.title === 'string' ? problem.title : undefined,
      detail: typeof problem.detail === 'string' ? problem.detail : undefined,
      fields: Array.isArray(problem.errors)
        ? problem.errors.map((item) => ({ field: item.field, code: item.code, message: item.message }))
        : [],
      requestId: typeof problem.request_id === 'string' ? problem.request_id : undefined,
      retryAfterSeconds: Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : undefined,
    });
  }
}

/** Сетевая ошибка: запрос не дошёл до сервера или прерван по таймауту. */
export class NetworkError extends Error {
  constructor(override readonly cause?: unknown) {
    super('Нет соединения');
    this.name = 'NetworkError';
  }
}

function httpFallbackCode(status: number): string {
  switch (status) {
    case 401:
      return 'UNAUTHENTICATED';
    case 403:
      return 'FORBIDDEN';
    case 404:
      return 'NOT_FOUND';
    case 409:
      return 'CONFLICT_VERSION';
    case 429:
      return 'RATE_LIMITED';
    case 503:
      return 'DEPENDENCY_UNAVAILABLE';
    default:
      return 'INTERNAL';
  }
}

/** Человеческое сообщение по коду ошибки (frontend-architecture §10). */
export function messageForError(error: unknown): string {
  if (error instanceof NetworkError) return 'Нет соединения. Проверьте сеть и повторите.';
  if (!(error instanceof ApiError)) return 'Что-то пошло не так. Повторите попытку.';
  switch (error.code) {
    case 'VALIDATION_FAILED':
      return error.detail ?? 'Проверьте заполненные поля.';
    case 'FORBIDDEN':
      return 'Недостаточно прав для этого действия.';
    case 'NOT_FOUND':
      return 'Объект не найден или недоступен.';
    case 'CONFLICT_VERSION':
      return 'Данные изменены другим участником. Обновите страницу.';
    case 'CONFLICT_ID_REUSED':
      return 'Повторите действие: идентификатор уже использован.';
    case 'QUOTA_EXCEEDED':
      return 'Превышено ограничение тарифа продукта.';
    case 'INVITE_EXPIRED':
      return 'Срок приглашения истёк — попросите новую ссылку.';
    case 'INVITE_INVALID':
      return 'Приглашение недействительно.';
    case 'ALREADY_MEMBER':
      return 'Вы уже участник этой организации.';
    case 'LINK_GONE':
      return 'Ссылка истекла — подготовьте файл заново.';
    case 'RATE_LIMITED':
    case 'OVERLOADED':
      return `Слишком много запросов, повторите через ${error.retryAfterSeconds ?? 60} с.`;
    case 'DEPENDENCY_UNAVAILABLE':
      return 'Сервис напоминаний временно недоступен — повторите через минуту.';
    case 'LAUNCH_DATA_EXPIRED':
      return 'Сессия запуска устарела — закройте и снова откройте приложение.';
    case 'LAUNCH_DATA_INVALID':
      return 'Не удалось подтвердить запуск из MAX.';
    default:
      return 'Что-то пошло не так. Повторите попытку.';
  }
}
