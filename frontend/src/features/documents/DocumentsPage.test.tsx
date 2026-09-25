import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { DocumentsPage } from './DocumentsPage';
import { ORG_ID, documentItems, me } from '../../test/fixtures';

const route = `/o/${ORG_ID}/documents`;
const path = '/o/:orgId/documents';

describe('T-FE-PAGE, T-FE-STATE: реестр документов', () => {
  it('показывает документы с сервера и состояние плана напоминаний', async () => {
    renderWithProviders(<DocumentsPage />, { route, path });
    expect(await screen.findByText('Лицензия на алкоголь')).toBeInTheDocument();
    expect(screen.getByText(/Договор аренды/)).toBeInTheDocument();
    expect(screen.getByText(/напоминания пересчитываются/)).toBeInTheDocument();
    expect(screen.getByText('Просрочен')).toBeInTheDocument();
  });

  it('фильтрует реестр по статусу через запрос к API', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DocumentsPage />, { route, path });
    await screen.findByText('Лицензия на алкоголь');
    await user.click(screen.getByRole('button', { name: 'Просрочено' }));
    await waitFor(() => expect(screen.queryByText('Лицензия на алкоголь')).not.toBeInTheDocument());
    expect(screen.getByText('Договор аренды')).toBeInTheDocument();
  });

  it('ищет по названию и показывает пустое состояние поиска', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DocumentsPage />, { route, path });
    await screen.findByText('Лицензия на алкоголь');
    await user.type(screen.getByLabelText('Поиск по названию'), 'страховка');
    expect(await screen.findByText('Ничего не найдено')).toBeInTheDocument();
  });

  it('показывает ошибку загрузки с кнопкой повтора', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/organizations/:organizationId/documents', () =>
        HttpResponse.json(
          { type: 'urn:vovremya:problem:internal', title: 'Сбой', status: 500, code: 'INTERNAL', request_id: 'r' },
          { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderWithProviders(<DocumentsPage />, { route, path });
    expect(await screen.findByRole('button', { name: 'Повторить' }, { timeout: 5000 })).toBeInTheDocument();
  });

  it('подгружает следующую страницу по курсору', async () => {
    const user = userEvent.setup();
    let page = 0;
    server.use(
      http.get('http://localhost:3000/api/v1/organizations/:organizationId/documents', () => {
        page += 1;
        if (page === 1) return HttpResponse.json({ items: [documentItems[0]], next_cursor: 'cursor-2' });
        return HttpResponse.json({ items: [documentItems[1]], next_cursor: null });
      }),
    );
    renderWithProviders(<DocumentsPage />, { route, path });
    await screen.findByText('Лицензия на алкоголь');
    await user.click(screen.getByRole('button', { name: 'Показать ещё' }));
    expect(await screen.findByText('Договор аренды')).toBeInTheDocument();
  });

  it('наблюдателю не показывает кнопку добавления документа', async () => {
    const viewer = {
      ...me,
      memberships: [{ ...me.memberships[0]!, role: 'viewer' as const }],
    };
    renderWithProviders(<DocumentsPage />, { route, path, me: viewer });
    await screen.findByText('Лицензия на алкоголь');
    expect(screen.queryByRole('button', { name: 'Добавить документ' })).not.toBeInTheDocument();
  });
});
