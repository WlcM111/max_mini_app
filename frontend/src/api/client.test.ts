import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { client, deleteCurrentSession, run } from './client';
import { ApiError, NetworkError, messageForError } from './errors';
import { server } from '../test/server-proxy';
import { calls } from '../test/msw/handlers';
import { getSession, setSession } from '../session/sessionStore';
import * as fixtures from '../test/fixtures';

const base = 'http://localhost:3000/api/v1';

describe('T-FE-API: поведение клиента публичного API', () => {
  it('добавляет заголовок авторизации к запросу', async () => {
    setSession({
      token: fixtures.session.token,
      expiresAt: fixtures.session.expires_at,
      account: fixtures.account,
      start: { kind: 'none' },
    });
    let authorization: string | null = null;
    server.use(
      http.get(`${base}/me`, ({ request }) => {
        authorization = request.headers.get('Authorization');
        return HttpResponse.json(fixtures.me);
      }),
    );
    await run(({ headers, signal }) => client.GET('/me', { headers, signal }));
    expect(authorization).toBe(`Bearer ${fixtures.session.token}`);
  });

  it('при 401 обновляет сессию и повторяет запрос ровно один раз', async () => {
    let attempts = 0;
    server.use(
      http.get(`${base}/me`, () => {
        attempts += 1;
        if (attempts === 1) {
          return HttpResponse.json(
            { type: 'urn:vovremya:problem:unauthenticated', title: 'Нужна сессия', status: 401, code: 'UNAUTHENTICATED', request_id: 'r' },
            { status: 401, headers: { 'Content-Type': 'application/problem+json' } },
          );
        }
        return HttpResponse.json(fixtures.me);
      }),
    );
    const me = await run(({ headers, signal }) => client.GET('/me', { headers, signal }));
    expect(me.account.id).toBe(fixtures.ACCOUNT_ID);
    expect(attempts).toBe(2);
    expect(calls.createSession).toBe(1);
    expect(getSession()?.token).toBe(fixtures.session.token);
  });

  it('повторный 401 после обновления сессии превращается в ApiError', async () => {
    server.use(
      http.get(`${base}/me`, () =>
        HttpResponse.json(
          { type: 'urn:vovremya:problem:launch-data-expired', title: 'Устарело', status: 401, code: 'LAUNCH_DATA_EXPIRED', request_id: 'r' },
          { status: 401, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    await expect(run(({ headers, signal }) => client.GET('/me', { headers, signal }))).rejects.toMatchObject({
      name: 'ApiError',
      status: 401,
      code: 'LAUNCH_DATA_EXPIRED',
    });
    expect(calls.createSession).toBe(1);
  });

  it('разбирает problem+json в ошибки полей', async () => {
    server.use(
      http.post(`${base}/organizations/:organizationId/documents`, () =>
        HttpResponse.json(
          {
            type: 'urn:vovremya:problem:validation-failed',
            title: 'Проверьте поля',
            status: 400,
            code: 'VALIDATION_FAILED',
            request_id: 'r',
            errors: [{ field: 'title', code: 'required', message: 'Введите название' }],
          },
          { status: 400, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    try {
      await run(({ headers, signal }) =>
        client.POST('/organizations/{organizationId}/documents', {
          params: { path: { organizationId: fixtures.ORG_ID } },
          body: { id: fixtures.DOC_ID, title: '' },
          headers,
          signal,
        }),
      );
      expect.unreachable('ожидалась ошибка');
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      const apiError = error as ApiError;
      expect(apiError.code).toBe('VALIDATION_FAILED');
      expect(apiError.fieldErrors().title).toBe('Введите название');
    }
  });

  it('читает Retry-After для ограничения частоты', async () => {
    server.use(
      http.get(`${base}/catalog`, () =>
        HttpResponse.json(
          { type: 'urn:vovremya:problem:rate-limited', title: 'Слишком часто', status: 429, code: 'RATE_LIMITED', request_id: 'r' },
          { status: 429, headers: { 'Content-Type': 'application/problem+json', 'Retry-After': '30' } },
        ),
      ),
    );
    await expect(run(({ headers, signal }) => client.GET('/catalog', { headers, signal }))).rejects.toMatchObject({
      retryAfterSeconds: 30,
    });
  });

  it('сетевой сбой превращается в NetworkError с понятным текстом', async () => {
    server.use(http.get(`${base}/catalog`, () => HttpResponse.error()));
    const failure = await run(({ headers, signal }) => client.GET('/catalog', { headers, signal })).catch((e) => e);
    expect(failure).toBeInstanceOf(NetworkError);
    expect(messageForError(failure)).toContain('Нет соединения');
  });

  it('выход очищает сессию даже при ошибке сервера', async () => {
    setSession({
      token: fixtures.session.token,
      expiresAt: fixtures.session.expires_at,
      account: fixtures.account,
      start: { kind: 'none' },
    });
    server.use(http.delete(`${base}/sessions/current`, () => new HttpResponse(null, { status: 500 })));
    await expect(deleteCurrentSession()).rejects.toBeInstanceOf(ApiError);
    expect(getSession()).toBeNull();
  });
});
