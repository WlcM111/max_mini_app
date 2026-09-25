import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { DocumentCardPage } from './DocumentCardPage';
import { DOC_ID, document as documentFixture, me } from '../../test/fixtures';

const route = `/d/${DOC_ID}`;
const path = '/d/:docId';

describe('T-FE-PAGE: карточка документа', () => {
  it('показывает статус, реквизиты и ближайшее напоминание', async () => {
    renderWithProviders(<DocumentCardPage />, { route, path });
    expect(await screen.findByRole('heading', { name: 'Лицензия на алкоголь' })).toBeInTheDocument();
    expect(screen.getByText(/Скоро истекает/)).toBeInTheDocument();
    expect(screen.getByText(/Ближайшее напоминание: 25.09.2026, 09:00/)).toBeInTheDocument();
    expect(screen.getByText(/АЛ-123/)).toBeInTheDocument();
    expect(screen.getByText(/Модельные данные справочника/)).toBeInTheDocument();
  });

  it('наблюдателю не показывает действий изменения', async () => {
    const viewer = { ...me, memberships: [{ ...me.memberships[0]!, role: 'viewer' as const }] };
    renderWithProviders(<DocumentCardPage />, { route, path, me: viewer });
    await screen.findByRole('heading', { name: 'Лицензия на алкоголь' });
    expect(screen.queryByRole('button', { name: 'Продлить' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Изменить' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Удалить' })).not.toBeInTheDocument();
  });

  it('сообщает, что план напоминаний недоступен', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/documents/:documentId', () =>
        HttpResponse.json({ ...documentFixture, reminders_state: 'unavailable', next_reminder_at: null }),
      ),
    );
    renderWithProviders(<DocumentCardPage />, { route, path });
    expect(await screen.findByText('План напоминаний временно недоступен')).toBeInTheDocument();
  });

  it('удалённый документ показывает понятное сообщение', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/documents/:documentId', () =>
        HttpResponse.json(
          { type: 'urn:vovremya:problem:not-found', title: 'Нет', status: 404, code: 'NOT_FOUND', request_id: 'r' },
          { status: 404, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderWithProviders(<DocumentCardPage />, { route, path });
    expect(await screen.findByText('Документ удалён или недоступен')).toBeInTheDocument();
  });
});
