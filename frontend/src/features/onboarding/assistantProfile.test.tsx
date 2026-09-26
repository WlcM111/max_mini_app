import { describe, expect, it } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { calls } from '../../test/msw/handlers';
import { OrganizationFormPage } from './OrganizationFormPage';
import { me } from '../../test/fixtures';
import { getDraft, resetDraft } from './onboardingDraft';

const route = '/onboarding/organization';
const base = 'http://localhost:3000/api/v1';

describe('T-FE-LLM: подбор профиля по описанию (FR-22)', () => {
  it('подставляет вид деятельности и сохраняет признаки в черновик', async () => {
    resetDraft();
    const user = userEvent.setup();
    renderWithProviders(<OrganizationFormPage />, { route, path: route });

    await user.type(await screen.findByLabelText('Описание бизнеса'), 'Кофейня, продаём пиво');
    await user.click(screen.getByRole('button', { name: 'Подобрать по описанию' }));

    await waitFor(() => expect(calls.matchProfile).toBe(1));
    expect(await screen.findByText(/Подобрано: Общественное питание, признаков: 2/)).toBeInTheDocument();
    expect(screen.getByLabelText('Вид деятельности')).toHaveValue('food_service');
    expect(getDraft().featureCodes).toEqual(['has_premises', 'sells_alcohol']);
  });

  it('сообщает, когда по описанию ничего не подобрано', async () => {
    resetDraft();
    const user = userEvent.setup();
    server.use(
      http.post(`${base}/profile-match`, () =>
        HttpResponse.json({ business_category_code: null, feature_codes: [], confidence: 0.1 }),
      ),
    );
    renderWithProviders(<OrganizationFormPage />, { route, path: route });

    await user.type(await screen.findByLabelText('Описание бизнеса'), 'что-то непонятное');
    await user.click(screen.getByRole('button', { name: 'Подобрать по описанию' }));

    expect(await screen.findByText(/ничего не подобрано/i)).toBeInTheDocument();
  });

  it('при выключенном ассистенте блок подбора скрыт', async () => {
    resetDraft();
    const withoutAssistant = { ...me, assistant_enabled: false };
    renderWithProviders(<OrganizationFormPage />, { route, path: route, me: withoutAssistant });

    await screen.findByLabelText('Название');
    expect(screen.queryByLabelText('Описание бизнеса')).not.toBeInTheDocument();
  });
});
