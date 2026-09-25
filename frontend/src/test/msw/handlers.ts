import { HttpResponse, http } from 'msw';
import * as fixtures from '../fixtures';

// msw/node требует абсолютные адреса: тесты выполняются в окружении Node.
const base = 'http://localhost:3000/api/v1';

/** Счётчики обращений: тесты проверяют отсутствие дублирующих запросов. */
export const calls = {
  createDocument: 0,
  createSession: 0,
  reset(): void {
    calls.createDocument = 0;
    calls.createSession = 0;
  },
};

function problem(status: number, code: string, detail: string, errors?: { field: string; code: string; message: string }[]) {
  return HttpResponse.json(
    {
      type: `urn:vovremya:problem:${code.toLowerCase()}`,
      title: detail,
      status,
      code,
      request_id: 'test-request',
      ...(errors ? { errors } : {}),
    },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  );
}

/** Обработчики всех операций, используемых мини-приложением. */
export const handlers = [
  http.post(`${base}/sessions`, () => {
    calls.createSession += 1;
    return HttpResponse.json(fixtures.session, { status: 201 });
  }),
  http.delete(`${base}/sessions/current`, () => new HttpResponse(null, { status: 204 })),
  http.get(`${base}/me`, () => HttpResponse.json(fixtures.me)),
  http.delete(`${base}/me`, () => new HttpResponse(null, { status: 204 })),
  http.get(`${base}/catalog`, () => HttpResponse.json(fixtures.catalog)),
  http.get(`${base}/organizations/:organizationId`, () => HttpResponse.json(fixtures.organization)),
  http.post(`${base}/organizations`, () => HttpResponse.json(fixtures.organization, { status: 201 })),
  http.patch(`${base}/organizations/:organizationId`, () => HttpResponse.json(fixtures.organization)),
  http.delete(`${base}/organizations/:organizationId`, () => new HttpResponse(null, { status: 204 })),
  http.get(`${base}/organizations/:organizationId/suggestions`, () =>
    HttpResponse.json({
      items: [
        {
          document_type_code: 'alcohol_license',
          title: 'Лицензия на розничную продажу алкоголя',
          description: 'Разрешение на продажу алкогольной продукции',
          data_status: 'model',
        },
      ],
    }),
  ),
  http.get(`${base}/organizations/:organizationId/documents`, ({ request }) => {
    const url = new URL(request.url);
    const status = url.searchParams.get('status');
    const query = url.searchParams.get('q');
    let items = fixtures.documentItems;
    if (status) items = items.filter((item) => item.status === status);
    if (query) items = items.filter((item) => item.title.toLowerCase().includes(query.toLowerCase()));
    return HttpResponse.json({ items, next_cursor: null });
  }),
  http.post(`${base}/organizations/:organizationId/documents`, async ({ request }) => {
    calls.createDocument += 1;
    const body = (await request.json()) as { title?: string };
    if (!body.title || body.title.trim() === '') {
      return problem(400, 'VALIDATION_FAILED', 'Проверьте поля', [
        { field: 'title', code: 'required', message: 'Введите название документа' },
      ]);
    }
    return HttpResponse.json({ ...fixtures.document, title: body.title }, { status: 201 });
  }),
  http.post(`${base}/organizations/:organizationId/documents/batch`, () =>
    HttpResponse.json({ items: [fixtures.document] }, { status: 201 }),
  ),
  http.get(`${base}/documents/:documentId`, () => HttpResponse.json(fixtures.document)),
  http.patch(`${base}/documents/:documentId`, () => HttpResponse.json({ ...fixtures.document, version: 2 })),
  http.delete(`${base}/documents/:documentId`, () => new HttpResponse(null, { status: 204 })),
  http.post(`${base}/documents/:documentId/renewals`, () => HttpResponse.json(fixtures.document, { status: 201 })),
  http.get(`${base}/organizations/:organizationId/members`, () => HttpResponse.json({ items: fixtures.members })),
  http.patch(`${base}/organizations/:organizationId/members/:accountId`, () =>
    HttpResponse.json({ ...fixtures.members[1], role: 'viewer' }),
  ),
  http.delete(`${base}/organizations/:organizationId/members/:accountId`, () => new HttpResponse(null, { status: 204 })),
  http.get(`${base}/organizations/:organizationId/invites`, () => HttpResponse.json({ items: [] })),
  http.post(`${base}/organizations/:organizationId/invites`, () =>
    HttpResponse.json(
      {
        id: '6c2b8e14-3f7a-4d95-8b20-1e4c6a9d3f07',
        role: 'editor',
        expires_at: '2026-09-27T09:00:00Z',
        link_url: `https://max.ru/vovremya_local_bot?startapp=inv_${'b'.repeat(43)}`,
        share_text: 'Приглашаю вести сроки документов «Кафе «Пример»» в приложении «Вовремя»',
      },
      { status: 201 },
    ),
  ),
  http.delete(`${base}/invites/:inviteId`, () => new HttpResponse(null, { status: 204 })),
  http.post(`${base}/invites/preview`, () =>
    HttpResponse.json({
      organization_name: 'Кафе «Пример»',
      role: 'editor',
      inviter_first_name: 'Михаил',
      expires_at: '2026-09-27T09:00:00Z',
    }),
  ),
  http.post(`${base}/invites/accept`, () => HttpResponse.json({ organization_id: fixtures.ORG_ID, role: 'editor' })),
  http.get(`${base}/organizations/:organizationId/notification-settings`, () =>
    HttpResponse.json({ enabled: true, local_time: '09:00' }),
  ),
  http.put(`${base}/organizations/:organizationId/notification-settings`, async ({ request }) =>
    HttpResponse.json(await request.json()),
  ),
  http.post(`${base}/organizations/:organizationId/exports/calendar`, () =>
    HttpResponse.json(
      {
        download_url: `/api/v1/downloads/${'c'.repeat(43)}`,
        file_name: 'vovremya-0b6f2c1e.ics',
        expires_at: '2026-09-24T09:10:00Z',
      },
      { status: 201 },
    ),
  ),
  http.post(`${base}/client-events`, () => new HttpResponse(null, { status: 202 })),
];

export { problem };
