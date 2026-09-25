import { client, run, type InviteAccepted, type InvitePreview } from '../../api/client';

/** Предпросмотр приглашения: организация, роль и пригласивший. */
export const previewInvite = (token: string): Promise<InvitePreview> =>
  run(({ headers, signal }) => client.POST('/invites/preview', { body: { token }, headers, signal }));

/** Принятие приглашения: пользователь становится участником организации. */
export const acceptInvite = (token: string): Promise<InviteAccepted> =>
  run(({ headers, signal }) => client.POST('/invites/accept', { body: { token }, headers, signal }));
