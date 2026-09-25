import { expect, test } from '@playwright/test';

// T-E2E-06: поведение при сетевых сбоях и отказах API.
test.describe('Сбои и восстановление', () => {
  test('показывает ошибку и кнопку повтора при недоступном API', async ({ page }) => {
    await page.route('**/api/v1/organizations/**', (route) => route.abort());
    await page.goto('/?mockUser=1001');
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible();
  });

  test('восстанавливается после возврата сети', async ({ page }) => {
    let failed = false;
    await page.route('**/api/v1/organizations/*', (route) => {
      if (!failed) {
        failed = true;
        return route.abort();
      }
      return route.continue();
    });
    await page.goto('/?mockUser=1001');
    await page.getByRole('button', { name: 'Повторить' }).click();
    await expect(page.getByRole('button', { name: 'Все документы' })).toBeVisible();
  });

  test('истёкшая сессия приводит к экрану перезапуска', async ({ page }) => {
    await page.route('**/api/v1/me', (route) =>
      route.fulfill({
        status: 401,
        contentType: 'application/problem+json',
        body: JSON.stringify({ status: 401, code: 'LAUNCH_DATA_EXPIRED', title: 'Устарело', request_id: 'e2e' }),
      }),
    );
    await page.goto('/?mockUser=1001');
    await expect(page.getByText(/Сессия запуска устарела|Не удалось начать работу/)).toBeVisible();
  });
});
