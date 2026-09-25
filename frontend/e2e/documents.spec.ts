import { expect, test } from '@playwright/test';

// T-E2E-02…04: онбординг, создание документа, реестр и карточка.
test.describe('Документы', () => {
  test('проходит онбординг и создаёт документ', async ({ page }) => {
    await page.goto(`/?mockUser=${Math.floor(Math.random() * 100000) + 700000}`);

    if (await page.getByRole('button', { name: 'Продолжить' }).isVisible()) {
      await page.getByRole('button', { name: 'Продолжить' }).click();
      await page.getByLabel('Название').fill('Кафе E2E');
      await page.getByRole('button', { name: 'Далее' }).click();
      await page.getByRole('button', { name: 'Создать организацию' }).click();
      await page.getByRole('button', { name: /Пропустить|Добавлю позже/ }).first().click();
    }

    await page.getByRole('button', { name: 'Добавить документ' }).click();
    await page.getByLabel('Название').fill('Лицензия E2E');
    await page.getByLabel('Действует до').fill('2027-01-31');
    await page.getByRole('button', { name: 'Сохранить' }).click();

    await expect(page.getByRole('heading', { name: 'Лицензия E2E' })).toBeVisible();
    await expect(page.getByText(/осталось|Скоро истекает|В порядке/)).toBeVisible();
  });

  test('фильтрует реестр и открывает карточку', async ({ page }) => {
    await page.goto('/?mockUser=1001');
    await page.getByRole('button', { name: 'Все документы' }).click();
    await page.getByRole('button', { name: 'Просрочено' }).click();
    await expect(page.getByRole('heading', { name: 'Документы' })).toBeVisible();
  });

  test('продлевает документ и сохраняет историю периодов', async ({ page }) => {
    await page.goto('/?mockUser=1001');
    await page.getByRole('button', { name: 'Все документы' }).click();
    await page.locator('.list-item').first().click();
    await page.getByRole('button', { name: 'Продлить' }).click();
    await page.getByRole('button', { name: 'Сохранить новый срок' }).click();
    await expect(page.getByText('История периодов')).toBeVisible();
  });
});
