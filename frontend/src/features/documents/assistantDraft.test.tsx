import { describe, expect, it, vi } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { calls, problem } from '../../test/msw/handlers';
import { DocumentFormPage } from './DocumentFormPage';
import { ORG_ID, me } from '../../test/fixtures';

// В jsdom нет декодера изображений: сжатие фото подменяется готовым результатом.
vi.mock('../../shared/lib/image', () => ({
  prepareImage: vi.fn(async () => ({ base64: 'AAAA', dataUrl: 'data:image/jpeg;base64,AAAA' })),
}));

const route = `/o/${ORG_ID}/documents/new`;
const path = '/o/:orgId/documents/new';
const base = 'http://localhost:3000/api/v1';

describe('T-FE-LLM: быстрый ввод документа по тексту (FR-21)', () => {
  it('заполняет форму распознанными полями и не сохраняет документ', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });

    await user.type(await screen.findByLabelText('Текст документа для распознавания'), 'Лицензия № 78РПА0012345 до 13.03.2029');
    await user.click(screen.getByRole('button', { name: 'Заполнить по тексту' }));

    await waitFor(() => expect(calls.draftDocument).toBe(1));
    expect(await screen.findByDisplayValue('Лицензия на алкоголь')).toBeInTheDocument();
    expect(screen.getByDisplayValue('78РПА0012345')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Комитет по промышленной политике')).toBeInTheDocument();
    expect(screen.getByLabelText('Действует до')).toHaveValue('13.03.2029');
    expect(screen.getByText(/Проверьте поля перед сохранением/)).toBeInTheDocument();
    expect(calls.createDocument).toBe(0);
  });

  it('дата выдачи из текста попадает в «Действует с», про окончание срока форма предупреждает', async () => {
    const user = userEvent.setup();
    server.use(
      http.post(`${base}/organizations/:organizationId/documents/draft`, () =>
        HttpResponse.json({
          title: 'Лицензия на Алкоголь',
          number: null,
          issuer: null,
          valid_from: '2022-03-12',
          valid_until: null,
          document_type_code: null,
          reminder_offsets_days: [],
          confidence: 0.7,
        }),
      ),
    );
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.type(await screen.findByLabelText('Текст документа для распознавания'), 'Лицензия на Алкоголь от 12.03.2022');
    await user.click(screen.getByRole('button', { name: 'Заполнить по тексту' }));

    expect(await screen.findByDisplayValue('Лицензия на Алкоголь')).toBeInTheDocument();
    expect(screen.getByLabelText('Действует с')).toHaveValue('12.03.2022');
    expect(screen.getByLabelText('Действует до')).toHaveValue('');
    expect(screen.getByRole('alert')).toHaveTextContent('Дата окончания в тексте не найдена');
  });

  it('при отказе ассистента показывает сообщение и оставляет ручной ввод', async () => {
    const user = userEvent.setup();
    server.use(
      http.post(`${base}/organizations/:organizationId/documents/draft`, () => problem(503, 'DEPENDENCY_UNAVAILABLE', 'Сервис временно недоступен')),
    );
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });

    await user.type(await screen.findByLabelText('Текст документа для распознавания'), 'любой текст');
    await user.click(screen.getByRole('button', { name: 'Заполнить по тексту' }));

    expect(await screen.findByText(/Ассистент сейчас недоступен/)).toBeInTheDocument();
    await user.type(screen.getByLabelText('Название'), 'Договор аренды');
    expect(screen.getByDisplayValue('Договор аренды')).toBeInTheDocument();
  });

  it('текст без реквизитов — понятная причина вместо общей ошибки', async () => {
    const user = userEvent.setup();
    server.use(
      http.post(`${base}/organizations/:organizationId/documents/draft`, () =>
        problem(422, 'DOCUMENT_NOT_RECOGNIZED', 'Реквизиты документа не распознаны'),
      ),
    );
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.type(await screen.findByLabelText('Текст документа для распознавания'), 'привет');
    await user.click(screen.getByRole('button', { name: 'Заполнить по тексту' }));
    expect(await screen.findByText(/В тексте не нашлось реквизитов документа/)).toBeInTheDocument();
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

describe('T-FE-LLM: распознавание по фото (FR-23)', () => {
  const photo = () => new File(['jpeg'], 'license.jpg', { type: 'image/jpeg' });

  it('заполняет форму по фото документа', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.upload(await screen.findByLabelText('Фото документа'), photo());

    await waitFor(() => expect(calls.draftImage).toBe(1));
    expect(await screen.findByDisplayValue('Лицензия на алкоголь')).toBeInTheDocument();
    expect(screen.getByLabelText('Действует до')).toHaveValue('13.03.2029');
    expect(screen.getByText(/Заполнено по фото/)).toBeInTheDocument();
    expect(screen.getByAltText('Фотография документа')).toBeInTheDocument();
  });

  it('неподходящее фото — причина и просьба приложить другой документ', async () => {
    const user = userEvent.setup();
    server.use(
      http.post(`${base}/organizations/:organizationId/documents/draft-image`, () =>
        problem(422, 'DOCUMENT_NOT_RECOGNIZED', 'Реквизиты документа не распознаны'),
      ),
    );
    renderWithProviders(<DocumentFormPage mode="create" />, { route, path });
    await user.upload(await screen.findByLabelText('Фото документа'), photo());
    expect(await screen.findByRole('alert')).toHaveTextContent('Документ на фото не подходит');
    expect(screen.getByRole('alert')).toHaveTextContent('Приложите фото');
  });
});
