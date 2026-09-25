import { createHmac } from 'node:crypto';

/**
 * Подписывает данные запуска MAX по алгоритму spec §4 (F-13): секрет —
 * HMAC-SHA256("WebAppData", токен бота), подпись — HMAC-SHA256(секрет, строка проверки).
 * Используется только в тестах с локальным тестовым токеном.
 */
export function signInitData(params: Record<string, string>, botToken: string): string {
  const keys = Object.keys(params).sort();
  const checkString = keys.map((key) => `${key}=${params[key] ?? ''}`).join('\n');
  const secret = createHmac('sha256', 'WebAppData').update(botToken).digest();
  const hash = createHmac('sha256', secret).update(checkString).digest('hex');
  const query = keys.map((key) => `${key}=${encodeURIComponent(params[key] ?? '')}`).join('&');
  return `${query}&hash=${hash}`;
}

/** Данные запуска пользователя MAX: идентификатор, имя и цель запуска. */
export function launchDataFor(userId: number, options: { startParam?: string; authDate?: number } = {}): string {
  const token = process.env.VOVREMYA_MOCK_BOT_TOKEN ?? 'devonly-local-bot-token';
  const params: Record<string, string> = {
    auth_date: String(options.authDate ?? Math.floor(Date.now() / 1000)),
    query_id: `int-${userId}-${Date.now()}`,
    user: JSON.stringify({
      id: userId,
      first_name: `Тест ${userId}`,
      last_name: null,
      username: null,
      language_code: 'ru',
      photo_url: null,
    }),
  };
  if (options.startParam) params.start_param = options.startParam;
  return signInitData(params, token);
}

export const apiBase = (): string =>
  process.env.VOVREMYA_API_BASE_URL ?? 'http://127.0.0.1:18170/api/v1';

export const botAdminBase = (): string =>
  process.env.VOVREMYA_BOT_ADMIN_URL ?? 'http://127.0.0.1:18181';
