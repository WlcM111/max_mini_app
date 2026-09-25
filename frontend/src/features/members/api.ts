import {
  client,
  run,
  type InviteCreated,
  type InviteSummary,
  type Member,
  type Role,
} from '../../api/client';

export const listMembers = (organizationId: string): Promise<Member[]> =>
  run(({ headers, signal }) =>
    client.GET('/organizations/{organizationId}/members', { params: { path: { organizationId } }, headers, signal }),
  ).then((result) => result.items);

export const updateMemberRole = (organizationId: string, accountId: string, role: Role): Promise<Member> =>
  run(({ headers, signal }) =>
    client.PATCH('/organizations/{organizationId}/members/{accountId}', {
      params: { path: { organizationId, accountId } },
      body: { role: role as 'editor' | 'viewer' },
      headers,
      signal,
    }),
  );

export const removeMember = (organizationId: string, accountId: string): Promise<void> =>
  run(({ headers, signal }) =>
    client.DELETE('/organizations/{organizationId}/members/{accountId}', {
      params: { path: { organizationId, accountId } },
      headers,
      signal,
    }),
  ).then(() => undefined);

export const listInvites = (organizationId: string): Promise<InviteSummary[]> =>
  run(({ headers, signal }) =>
    client.GET('/organizations/{organizationId}/invites', { params: { path: { organizationId } }, headers, signal }),
  ).then((result) => result.items);

/** Ссылка приглашения возвращается один раз — только в ответе на создание. */
export const createInvite = (organizationId: string, id: string, role: 'editor' | 'viewer'): Promise<InviteCreated> =>
  run(({ headers, signal }) =>
    client.POST('/organizations/{organizationId}/invites', {
      params: { path: { organizationId } },
      body: { id, role },
      headers,
      signal,
    }),
  );

export const revokeInvite = (inviteId: string): Promise<void> =>
  run(({ headers, signal }) =>
    client.DELETE('/invites/{inviteId}', { params: { path: { inviteId } }, headers, signal }),
  ).then(() => undefined);
