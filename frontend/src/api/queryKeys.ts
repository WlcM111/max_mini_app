// Ключи кэша серверного состояния (frontend-architecture §6).
export const queryKeys = {
  me: () => ['me'] as const,
  catalog: () => ['catalog'] as const,
  organization: (organizationId: string) => ['org', organizationId] as const,
  suggestions: (organizationId: string) => ['suggestions', organizationId] as const,
  documents: (organizationId: string, filter: { status?: string; q?: string; limit?: number; responsible?: string }) =>
    ['documents', organizationId, filter] as const,
  document: (documentId: string) => ['document', documentId] as const,
  members: (organizationId: string) => ['members', organizationId] as const,
  invites: (organizationId: string) => ['invites', organizationId] as const,
  notify: (organizationId: string) => ['notify', organizationId] as const,
};
