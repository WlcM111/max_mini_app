import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { SessionContext } from '../../session/useSession';
import { setSession } from '../../session/sessionStore';
import { calls } from '../../test/msw/handlers';
import * as fixtures from '../../test/fixtures';
import { DatesPage } from './DatesPage';
import { SuggestionsPage } from './SuggestionsPage';

const documentsPath = `/o/${fixtures.ORG_ID}/documents`;

// Раздел «Документы» → подбор → сроки: после сохранения «Назад» ведёт в реестр.
function renderFlow() {
  const session = { token: fixtures.session.token, expiresAt: fixtures.session.expires_at, account: fixtures.account, start: { kind: 'none' as const } };
  setSession(session);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
  const route = (path: string, element: ReactNode) => ({ path, element });
  const router = createMemoryRouter(
    [
      route('/o/:orgId/documents', <p>Реестр документов</p>),
      route('/o/:orgId/documents/typical', <SuggestionsPage />),
      route('/o/:orgId/documents/typical/dates', <DatesPage />),
    ],
    { initialEntries: [documentsPath, `${documentsPath}/typical`], initialIndex: 1 },
  );
  const value = { session, me: fixtures.me, platform: 'web', bridgeKind: 'mock' as const, refreshMe: async () => undefined };
  return render(
    <QueryClientProvider client={queryClient}>
      <SessionContext.Provider value={value}>
        <RouterProvider router={router} />
      </SessionContext.Provider>
    </QueryClientProvider>,
  );
}

describe('T-FE-TYPICAL: подбор типовых и своих документов в разделе «Документы» (FR-27)', () => {
  it('сохраняет типовой и свой документ одним пакетом и возвращает в реестр', async () => {
    const user = userEvent.setup();
    renderFlow();
    await user.click(await screen.findByRole('button', { name: /Лицензия на розничную продажу алкоголя/ }));
    await user.type(screen.getByLabelText('Название своего документа'), 'Договор с поставщиком');
    await user.click(screen.getByRole('button', { name: 'Добавить' }));
    expect(screen.getByText('Договор с поставщиком')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Далее (2)' }));
    expect(await screen.findByRole('heading', { name: 'Сроки документов' })).toBeInTheDocument();
    expect(screen.getByText('Свой документ')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Сохранить документы (2)' }));
    expect(await screen.findByText('Реестр документов')).toBeInTheDocument();
    expect(calls.batches).toHaveLength(1);
    expect(calls.batches[0]).toEqual([
      expect.objectContaining({ document_type_code: 'alcohol_license', title: 'Лицензия на розничную продажу алкоголя', reminder_offsets_days: [60, 30, 7] }),
      expect.objectContaining({ document_type_code: null, title: 'Договор с поставщиком', reminder_offsets_days: [30, 7, 1] }),
    ]);
  });

  it('не добавляет пустые и повторяющиеся названия, свой документ можно убрать', async () => {
    const user = userEvent.setup();
    renderFlow();
    const input = await screen.findByLabelText('Название своего документа');
    await user.click(screen.getByRole('button', { name: 'Добавить' }));
    expect(screen.getByText('Введите название (до 200 символов)')).toBeInTheDocument();

    await user.type(input, 'Договор с поставщиком{Enter}');
    await user.type(input, 'договор С поставщиком{Enter}');
    expect(screen.getByText('Такой документ уже в списке')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Убрать «Договор с поставщиком»' }));
    expect(screen.getByRole('button', { name: 'Готово' })).toBeInTheDocument();
  });
});
