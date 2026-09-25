import {
  client,
  run,
  type Document,
  type DocumentCreate,
  type DocumentPage,
  type DocumentUpdate,
  type RenewalCreate,
} from '../../api/client';

export interface DocumentsFilter {
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
        },
      },
      headers,
      signal,
    }),
  );

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
