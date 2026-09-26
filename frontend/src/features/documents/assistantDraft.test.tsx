import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { calls } from '../../test/msw/handlers';
import { DocumentFormPage } from './DocumentFormPage';
import { ORG_ID, me } from '../../test/fixtures';

const route = `/o/${ORG_ID}/documents/new`;
const path = '/o/:orgId/documents/new';
const base = 'http://localhost:3000/api/v1';

describe('T-FE-LLM: быстрый ввод документа по тексту (FR-21)', () => {
  it('заполняет форму распознанными полями и не сохраняет документ', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });

    await user.type(
      await screen.findByLabelText('Текст документа для распознавания'),
      'Лицензия № 78РПА0012345 до 13.03.2029',
    );
    await user.click(screen.getByRole('button', { name: 'Заполнить по тексту' }));

    await waitFor(() => expect(calls.draftDocument).toBe(1));
    expect(await screen.findByDisplayValue('Лицензия на алкоголь')).toBeInTheDocument();
    expect(screen.getByDisplayValue('78РПА0012345')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Комитет по промышленной политике')).toBeInTheDocument();
    expect(screen.getByLabelText('Действует до')).toHaveValue('2029-03-13');
    expect(screen.getByText(/проверьте их перед сохранением/i)).toBeInTheDocument();
    expect(calls.createDocument).toBe(0);
  });

  it('при отказе ассистента показывает сообщение и оставляет ручной ввод', async () => {
    const user = userEvent.setup();
    server.use(
      http.post(`${base}/organizations/:organizationId/documents/draft`, () =>
        HttpResponse.json(
          {
            type: 'urn:vovremya:problem:dependency-unavailable',
            title: 'Сервис временно недоступен',
            status: 503,
            code: 'DEPENDENCY_UNAVAILABLE',
            request_id: 'r',
          },
          { status: 503, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });

    await user.type(await screen.findByLabelText('Текст документа для распознавания'), 'любой текст');
    await user.click(screen.getByRole('button', { name: 'Заполнить по тексту' }));

    expect(await screen.findByText(/Сервис напоминаний временно недоступен|временно недоступен/i)).toBeInTheDocument();
    await user.type(screen.getByLabelText('Название'), 'Договор аренды');
    expect(screen.getByDisplayValue('Договор аренды')).toBeInTheDocument();
  });

  it('при выключенном ассистенте блок быстрого ввода скрыт', async () => {
    const withoutAssistant = { ...me, assistant_enabled: false };
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path, me: withoutAssistant });

    await screen.findByLabelText('Название');
    expect(screen.queryByLabelText('Текст документа для распознавания')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Заполнить по тексту' })).not.toBeInTheDocument();
  });

  it('кнопка недоступна, пока текст не введён', async () => {
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    expect(await screen.findByRole('button', { name: 'Заполнить по тексту' })).toBeDisabled();
  });
});
