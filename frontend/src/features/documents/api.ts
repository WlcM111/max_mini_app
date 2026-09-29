import {
  client,
  run,
  type Document,
  type DocumentDraft,
  type DocumentCreate,
  type DocumentPage,
  type DocumentUpdate,
  type RenewalCreate,
} from '../../api/client';

export interface DocumentsFilter {
  /** me — только документы, где ответственный — текущий пользователь */
  responsible?: string;
  status?: string;
  q?: string;
  cursor?: string;
  limit?: number;
}

/** Страница реестра документов организации (сортировка и курсор — на сервере). */
export const listDocuments = (organizationId: string, filter: DocumentsFilter = {}): Promise<DocumentPage> =>
  run(({ headers, signal }) =>
    client.GET('/organizations/{organizationId}/documents', {
      params: {
        path: { organizationId },
        query: {
          ...(filter.status ? { status: filter.status as never } : {}),
          ...(filter.q ? { q: filter.q } : {}),
          ...(filter.cursor ? { cursor: filter.cursor } : {}),
          ...(filter.limit ? { limit: filter.limit } : {}),
          ...(filter.responsible ? { responsible: filter.responsible } : {}),
        },
      },
      headers,
      signal,
    }),
  );

/** Названия всех документов организации (страницы по 100) — для проверки повторов при импорте. */
export async function listDocumentTitles(organizationId: string): Promise<string[]> {
  const titles: string[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < 10; page += 1) {
    const result = await listDocuments(organizationId, { limit: 100, ...(cursor ? { cursor } : {}) });
    titles.push(...result.items.map((item) => item.title));
    if (!result.next_cursor) break;
    cursor = result.next_cursor;
  }
  return titles;
}

export const getDocument = (documentId: string): Promise<Document> =>
  run(({ headers, signal }) =>
    client.GET('/documents/{documentId}', { params: { path: { documentId } }, headers, signal }),
  );

export const createDocument = (organizationId: string, body: DocumentCreate): Promise<Document> =>
  run(({ headers, signal }) =>
    client.POST('/organizations/{organizationId}/documents', {
      params: { path: { organizationId } },
      body,
      headers,
      signal,
    }),
  );

export const createDocumentsBatch = (organizationId: string, items: DocumentCreate[]): Promise<Document[]> =>
  run(({ headers, signal }) =>
    client.POST('/organizations/{organizationId}/documents/batch', {
      params: { path: { organizationId } },
      body: { items },
      headers,
      signal,
    }),
  ).then((result) => result.items);

export const updateDocument = (documentId: string, body: DocumentUpdate): Promise<Document> =>
  run(({ headers, signal }) =>
    client.PATCH('/documents/{documentId}', { params: { path: { documentId } }, body, headers, signal }),
  );

export const renewDocument = (documentId: string, body: RenewalCreate): Promise<Document> =>
  run(({ headers, signal }) =>
    client.POST('/documents/{documentId}/renewals', { params: { path: { documentId } }, body, headers, signal }),
  );

export const deleteDocument = (documentId: string): Promise<void> =>
  run(({ headers, signal }) =>
    client.DELETE('/documents/{documentId}', { params: { path: { documentId } }, headers, signal }),
  ).then(() => undefined);

/**
 * Черновик карточки документа из свободного текста (FR-21). Ничего не сохраняет:
 * поля подставляются в форму, которую подтверждает пользователь.
 */
/** Черновик по фотографии: изображение уходит в GigaChat и там сразу удаляется. */
// Ожидание дольше серверного бюджета ассистента (ADR-033): текст — до 13 с, фото — до 29 с.
const DRAFT_TIMEOUT_MS = 20_000;
const DRAFT_IMAGE_TIMEOUT_MS = 60_000;

export const draftDocumentFromImage = (organizationId: string, image: string, mimeType: 'image/jpeg' | 'image/png'): Promise<DocumentDraft> =>
  run(
    ({ headers, signal }) =>
      client.POST('/organizations/{organizationId}/documents/draft-image', {
        params: { path: { organizationId } },
        body: { image, mime_type: mimeType },
        headers,
        signal,
      }),
    { timeoutMs: DRAFT_IMAGE_TIMEOUT_MS },
  );

export const draftDocument = (organizationId: string, text: string): Promise<DocumentDraft> =>
  run(
    ({ headers, signal }) =>
      client.POST('/organizations/{organizationId}/documents/draft', {
        params: { path: { organizationId } },
        body: { text },
        headers,
        signal,
      }),
    { timeoutMs: DRAFT_TIMEOUT_MS },
  );
