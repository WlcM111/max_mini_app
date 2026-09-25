import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { InviteAcceptPage } from './InviteAcceptPage';

describe('T-FE-PAGE: приём приглашения', () => {
  it('показывает организацию, роль и пригласившего', async () => {
    renderWithProviders(<InviteAcceptPage token="inv-token" />, { route: '/invite', path: '/invite' });
    expect(await screen.findByRole('heading', { name: 'Кафе «Пример»' })).toBeInTheDocument();
    expect(screen.getByText(/Михаил приглашает вас/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Принять приглашение' })).toBeInTheDocument();
  });

  it('сообщает о недействительном приглашении', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/invites/preview', () =>
        HttpResponse.json(
          { type: 'urn:vovremya:problem:invite-expired', title: 'Истекло', status: 410, code: 'INVITE_EXPIRED', request_id: 'r' },
          { status: 410, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderWithProviders(<InviteAcceptPage token="inv-token" />, { route: '/invite', path: '/invite' });
    expect(await screen.findByText('Приглашение недействительно')).toBeInTheDocument();
    expect(screen.getByText(/Срок приглашения истёк/)).toBeInTheDocument();
  });

  it('без токена предлагает вернуться на главную', async () => {
    const user = userEvent.setup();
    renderWithProviders(<InviteAcceptPage />, { route: '/invite', path: '/invite' });
    expect(await screen.findByText(/Ссылка приглашения недействительна/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'На главную' }));
  });
});
