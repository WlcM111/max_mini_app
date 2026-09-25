import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { calls } from '../../test/msw/handlers';
import { DocumentFormPage } from './DocumentFormPage';
import { ORG_ID, document as documentFixture } from '../../test/fixtures';

const route = `/o/${ORG_ID}/documents/new`;
const path = '/o/:orgId/documents/new';

describe('T-FE-DUP, T-FE-PAGE: форма документа', () => {
  it('двойное нажатие «Сохранить» отправляет один запрос', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.type(await screen.findByLabelText('Название'), 'Новая лицензия');
    const button = screen.getByRole('button', { name: 'Сохранить' });
    await user.dblClick(button);
    await waitFor(() => expect(calls.createDocument).toBe(1));
  });

  it('повтор после сетевой ошибки использует тот же идентификатор', async () => {
    const user = userEvent.setup();
    const ids: string[] = [];
    let attempt = 0;
    server.use(
      http.post('http://localhost:3000/api/v1/organizations/:organizationId/documents', async ({ request }) => {
        const body = (await request.json()) as { id: string };
        ids.push(body.id);
        attempt += 1;
        if (attempt === 1) return HttpResponse.error();
        return HttpResponse.json({ ...documentFixture, id: body.id }, { status: 201 });
      }),
    );
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.type(await screen.findByLabelText('Название'), 'Договор');
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    await screen.findByText(/Нет соединения/);
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(ids).toHaveLength(2));
    expect(ids[0]).toBe(ids[1]);
  });

  it('показывает ошибку поля, пришедшую от сервера', async () => {
    const user = userEvent.setup();
    server.use(
      http.post('http://localhost:3000/api/v1/organizations/:organizationId/documents', () =>
        HttpResponse.json(
          {
            type: 'urn:vovremya:problem:validation-failed',
            title: 'Проверьте поля',
            status: 400,
            code: 'VALIDATION_FAILED',
            request_id: 'r',
            errors: [{ field: 'reference_url', code: 'invalid', message: 'Ссылка недоступна' }],
          },
          { status: 400, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.type(await screen.findByLabelText('Название'), 'Лицензия');
    await user.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Ссылка недоступна')).toBeInTheDocument();
  });

  it('не отправляет форму с пустым названием', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.click(await screen.findByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Введите название (до 200 символов)')).toBeInTheDocument();
    expect(calls.createDocument).toBe(0);
  });
});
