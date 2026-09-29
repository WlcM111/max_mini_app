import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { calls, problem } from '../../test/msw/handlers';
import { ImportPage } from './ImportPage';
import { ORG_ID } from '../../test/fixtures';

const route = `/o/${ORG_ID}/documents/import`;
const path = '/o/:orgId/documents/import';
const base = 'http://localhost:3000/api/v1';

function csvFile(text: string, name = 'reestr.csv'): File {
  const bytes = new TextEncoder().encode(text);
  const file = new File([bytes], name, { type: 'text/csv' });
  if (typeof file.arrayBuffer !== 'function') {
    Object.defineProperty(file, 'arrayBuffer', { value: async () => bytes.slice().buffer });
  }
  return file;
}

const manyRows = (count: number) =>
  ['Название;Действует до', ...Array.from({ length: count }, (_, index) => `Документ ${index + 1};31.12.2027`)].join('\n');

describe('T-FE-IMPORT: импорт документов из таблицы (FR-24)', () => {
  it('показывает разбор файла и загружает только корректные строки', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ImportPage />, { route, path });
    await user.upload(screen.getByLabelText('Выбрать файл'), csvFile('Название;Действует до\nЛицензия на алкоголь;13.03.2029\n;01.01.2027\nУстав;бессрочно\n'));

    expect(await screen.findByText('reestr.csv')).toBeInTheDocument();
    expect(screen.getByText('Строк: 3. Готово к загрузке: 2. С ошибками: 1.')).toBeInTheDocument();
    expect(screen.getByText('Введите название (до 200 символов)')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Импортировать 2 документа' }));
    expect(await screen.findByRole('button', { name: 'К документам' })).toBeInTheDocument();
    expect(calls.batches).toHaveLength(1);
    expect(calls.batches[0]?.map((item) => item.title)).toEqual(['Лицензия на алкоголь', 'Устав']);
  });

  it('делит загрузку на пакеты по 30 документов', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ImportPage />, { route, path });
    await user.upload(screen.getByLabelText('Выбрать файл'), csvFile(manyRows(31)));
    await user.click(await screen.findByRole('button', { name: 'Импортировать 31 документ' }));
    expect(await screen.findByRole('button', { name: 'К документам' })).toBeInTheDocument();
    expect(calls.batches.map((batch) => batch.length)).toEqual([30, 1]);
  });

  it('после сбоя догружает только оставшиеся строки с прежними идентификаторами', async () => {
    const user = userEvent.setup();
    const sent: string[][] = [];
    let attempt = 0;
    server.use(
      http.post(`${base}/organizations/:organizationId/documents/batch`, async ({ request }) => {
        attempt += 1;
        const body = (await request.json()) as { items: { id: string }[] };
        sent.push(body.items.map((item) => item.id));
        if (attempt === 2) return problem(500, 'INTERNAL', 'Сбой');
        return HttpResponse.json({ items: [] }, { status: 201 });
      }),
    );
    renderWithProviders(<ImportPage />, { route, path });
    await user.upload(screen.getByLabelText('Выбрать файл'), csvFile(manyRows(31)));
    await user.click(await screen.findByRole('button', { name: 'Импортировать 31 документ' }));

    // Текст о прогрессе есть и в подсказке поля, и в сообщении об ошибке —
    // проверяем именно сообщение, иначе совпадений несколько.
    expect(await screen.findByRole('alert')).toHaveTextContent(/Загружено 30 из 31/);
    await user.click(screen.getByRole('button', { name: 'Импортировать 1 документ' }));
    expect(await screen.findByRole('button', { name: 'К документам' })).toBeInTheDocument();
    expect(sent.map((ids) => ids.length)).toEqual([30, 1, 1]);
    expect(sent[2]).toEqual(sent[1]);
  });
});
