import { expect, test } from '@playwright/test';

// T-E2E-05: участники и приглашения.
test.describe('Участники', () => {
  test('создаёт приглашение и показывает ссылку один раз', async ({ page }) => {
    await page.goto('/?mockUser=1001');
    await page.getByRole('button', { name: 'Участники' }).click();
    await page.getByRole('button', { name: 'Пригласить' }).click();
    await page.getByRole('button', { name: 'Создать приглашение' }).click();
    await expect(page.getByText(/startapp=inv_/)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Отправить в MAX' })).toBeVisible();
  });

  test('принимает приглашение по диплинку вторым пользователем', async ({ page, context }) => {
    await page.goto('/?mockUser=1001');
    await page.getByRole('button', { name: 'Участники' }).click();
    await page.getByRole('button', { name: 'Пригласить' }).click();
    await page.getByRole('button', { name: 'Создать приглашение' }).click();
    const link = await page.getByText(/startapp=inv_/).innerText();
    const token = new URL(link.trim()).searchParams.get('startapp');

    const guest = await context.newPage();
    await guest.goto(`/?mockUser=800001&startapp=${token}`);
    await expect(guest.getByRole('button', { name: 'Принять приглашение' })).toBeVisible();
    await guest.getByRole('button', { name: 'Принять приглашение' }).click();
    await expect(guest.getByRole('button', { name: 'Все документы' })).toBeVisible();
  });
});
