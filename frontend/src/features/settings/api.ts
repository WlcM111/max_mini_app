import { client, run, type NotificationSettings } from '../../api/client';

export const getNotificationSettings = (organizationId: string): Promise<NotificationSettings> =>
  run(({ headers, signal }) =>
    client.GET('/organizations/{organizationId}/notification-settings', {
      params: { path: { organizationId } },
      headers,
      signal,
    }),
  );

export const putNotificationSettings = (
  organizationId: string,
  body: NotificationSettings,
): Promise<NotificationSettings> =>
  run(({ headers, signal }) =>
    client.PUT('/organizations/{organizationId}/notification-settings', {
      params: { path: { organizationId } },
      body,
      headers,
      signal,
    }),
  );

/** Удаление аккаунта вместе с организациями, где пользователь владелец. */
export const deleteAccount = (): Promise<void> =>
  run(({ headers, signal }) => client.DELETE('/me', { headers, signal })).then(() => undefined);
