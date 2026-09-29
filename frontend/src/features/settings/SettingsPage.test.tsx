import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { SettingsPage } from './SettingsPage';
import { ORG_ID } from '../../test/fixtures';

describe('T-FE-PAGE: настройки напоминаний', () => {
  it('сохраняет время напоминаний, выбранное в сетке значений', async () => {
    const user = userEvent.setup();
    type NotificationSettings = { enabled: boolean; local_time: string };
    let saved: NotificationSettings | null = null;
    server.use(
      http.put('http://localhost:3000/api/v1/organizations/:organizationId/notification-settings', async ({ request }) => {
        saved = (await request.json()) as NotificationSettings;
        return HttpResponse.json(saved);
      }),
    );
    renderWithProviders(<SettingsPage />, { route: `/o/${ORG_ID}/settings`, path: '/o/:orgId/settings' });
    const field = await screen.findByLabelText('Время напоминаний');
    await waitFor(() => expect(field).toHaveValue('09:00'));

    await user.click(field);
    await user.click(within(screen.getByRole('dialog', { name: 'Время напоминаний' })).getByRole('button', { name: '18:30' }));
    await waitFor(() => expect(saved).toEqual({ enabled: true, local_time: '18:30' }));
    expect(field).toHaveValue('18:30');
  });
});
