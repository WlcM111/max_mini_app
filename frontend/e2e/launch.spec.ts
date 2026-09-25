import { expect, test } from '@playwright/test';

// T-E2E-01: запуск мини-приложения и первый экран.
test.describe('Запуск приложения', () => {
  test('без окружения MAX показывает экран «Откройте из MAX»', async ({ page }) => {
    await page.goto('/?mockBridge=off');
    await expect(page.getByRole('heading', { name: /Откройте приложение из чата с ботом в MAX/ })).toBeVisible();
  });

  test('с имитацией MAX доходит до дашборда или онбординга', async ({ page }) => {
    await page.goto('/?mockUser=1001');
    await expect(page.getByText('Имитация MAX')).toBeVisible();
    await expect(page.getByRole('heading').first()).toBeVisible();
    const onboarding = page.getByRole('button', { name: 'Продолжить' });
    const dashboard = page.getByRole('button', { name: 'Все документы' });
    await expect(onboarding.or(dashboard)).toBeVisible();
  });

  test('телеметрия запуска не содержит данных запуска', async ({ page }) => {
    const bodies: string[] = [];
    page.on('request', (request) => {
      if (request.url().includes('/client-events')) bodies.push(request.postData() ?? '');
    });
    await page.goto('/?mockUser=1001');
    await page.waitForTimeout(2500);
    for (const body of bodies) {
      expect(body).not.toContain('hash=');
      expect(body).not.toContain('auth_date');
    }
  });
});
