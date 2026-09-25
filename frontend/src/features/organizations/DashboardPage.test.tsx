import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { DashboardPage } from './DashboardPage';
import { ORG_ID, me, organization } from '../../test/fixtures';

const route = `/o/${ORG_ID}`;
const path = '/o/:orgId';

describe('T-FE-PAGE, T-FE-STATE: дашборд организации', () => {
  it('показывает счётчики статусов и ближайшие сроки', async () => {
    renderWithProviders(<DashboardPage />, { route, path });
    expect(await screen.findByRole('heading', { name: 'Кафе «Пример»' })).toBeInTheDocument();
    expect(screen.getByText('Просрочено').previousSibling).toHaveTextContent('1');
    expect(await screen.findByText('Лицензия на алкоголь')).toBeInTheDocument();
  });

  it('предупреждает, когда канал напоминаний не активен', async () => {
    const stopped = {
      ...me,
      reminders_channel: { state: 'stopped' as const, bot_chat_url: 'https://max.ru/vovremya_local_bot' },
    };
    renderWithProviders(<DashboardPage />, { route, path, me: stopped });
    expect(await screen.findByText(/Напоминания не приходят/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Открыть чат с ботом' })).toBeInTheDocument();
  });

  it('показывает пустое состояние с предложением добавить документ', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/organizations/:organizationId', () =>
        HttpResponse.json({
          ...organization,
          stats: { total: 0, expired: 0, expiring: 0, valid: 0, no_expiry: 0, next_valid_until: null },
        }),
      ),
      http.get('http://localhost:3000/api/v1/organizations/:organizationId/documents', () =>
        HttpResponse.json({ items: [], next_cursor: null }),
      ),
    );
    renderWithProviders(<DashboardPage />, { route, path });
    expect(await screen.findByText('Документов пока нет')).toBeInTheDocument();
  });
});
