import { expect, test, type Page } from '@playwright/test';

// Проверка вёрстки и сравнение скриншотов. API подменяется теми же MSW-обработчиками и фикстурами,
// что в юнит-тестах, время заморожено — снимки не зависят от сегодняшней даты и бэкенда.
import { getResponse } from 'msw';
import { handlers } from '../../src/test/msw/handlers';

// Обработчики MSW заданы для http://localhost:3000 — запросы приложения переадресуются туда.
const MOCK_ORIGIN = 'http://localhost:3000';

// Правила вёрстки: ничего за краем, без наложений, без обрезанного текста, контент не под нижними панелями.
const LAYOUT_CHECK = `(() => {
  const vw = document.documentElement.clientWidth;
  const problems = [];
  const hidden = (el) => { for (let e = el; e && e !== document.body; e = e.parentElement) { const cs = getComputedStyle(e);
    if (cs.display === 'none' || cs.visibility === 'hidden' || parseFloat(cs.opacity) < 0.05 || e.hasAttribute('data-decor') || e.classList.contains('visually-hidden')) return true; } return false; };
  const layer = (el) => { for (let e = el; e && e !== document.body; e = e.parentElement) { const p = getComputedStyle(e).position; if (p === 'fixed' || p === 'sticky') return e; } return null; };
  const scroller = (el) => { for (let e = el.parentElement; e && e !== document.body; e = e.parentElement) { const cs = getComputedStyle(e);
    if (/(auto|scroll)/.test(cs.overflowX) && e.scrollWidth > e.clientWidth + 1) return e; } return null; };
  const atoms = [];
  for (const el of document.body.querySelectorAll('*')) {
    const tag = el.tagName.toUpperCase();
    if (['PATH', 'RECT', 'OPTION', 'CIRCLE', 'G'].includes(tag)) continue;
    const text = [...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim());
    if (!text && !['BUTTON', 'INPUT', 'SELECT', 'TEXTAREA', 'SVG', 'A'].includes(tag)) continue;
    if (hidden(el)) continue;
    const r = el.getBoundingClientRect();
    if (r.width < 1 || r.height < 1) continue;
    atoms.push({ el, r, layer: layer(el), tag });
  }
  const name = (el) => (el.getAttribute('aria-label') || el.textContent || el.tagName).trim().slice(0, 40);
  for (const a of atoms) {
    if ((a.r.right > vw + 1 || a.r.left < -1) && !scroller(a.el)) problems.push('за краем: ' + name(a.el));
    if (a.tag === 'BUTTON' && a.el.scrollWidth > a.el.clientWidth + 1) problems.push('обрезана кнопка: ' + name(a.el));
  }
  for (let i = 0; i < atoms.length; i++) for (let j = i + 1; j < atoms.length; j++) {
    const a = atoms[i], b = atoms[j];
    if (a.layer !== b.layer || a.el.contains(b.el) || b.el.contains(a.el)) continue;
    const x = Math.min(a.r.right, b.r.right) - Math.max(a.r.left, b.r.left), y = Math.min(a.r.bottom, b.r.bottom) - Math.max(a.r.top, b.r.top);
    const ca = a.el.closest('.control');
    if (x > 2 && y > 2 && !(ca && ca === b.el.closest('.control'))) problems.push('наложение: ' + name(a.el) + ' / ' + name(b.el));
  }
  return [...new Set(problems)];
})()`;

async function mockApi(page: Page) {
  await page.route('**/api/v1/**', async (route) => {
    const req = route.request();
    const body = req.postData();
    const url = new URL(req.url());
    const request = new Request(MOCK_ORIGIN + url.pathname + url.search, { method: req.method(), headers: req.headers(), body: body ?? undefined });
    const response = await getResponse(handlers, request);
    if (!response) return route.fulfill({ status: 404, contentType: 'application/problem+json', body: '{"status":404}' });
    return route.fulfill({
      status: response.status,
      headers: Object.fromEntries(response.headers.entries()),
      body: await response.text(),
    });
  });
}

// Экран и как до него дойти от главной: кнопка нижней навигации и заголовок раздела.
const SCREENS: { name: string; tab?: string; extra?: string; heading?: string }[] = [
  { name: 'dashboard' },
  { name: 'documents', tab: 'Документы', heading: 'Документы' },
  { name: 'calendar', tab: 'Документы', extra: 'Календарь', heading: 'Документы' },
  { name: 'members', tab: 'Участники', heading: 'Участники' },
  { name: 'settings', tab: 'Настройки', heading: 'Настройки' },
];

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date('2026-09-28T10:00:00+03:00'));
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await mockApi(page);
});

for (const screen of SCREENS) {
  test(`вёрстка и скриншот: ${screen.name}`, async ({ page }) => {
    await page.goto('/?mockUser=1001');
    await page.getByText('Ближайшие сроки').first().waitFor();
    if (screen.tab) await page.getByRole('button', { name: screen.tab, exact: true }).click();
    if (screen.heading) await page.getByRole('heading', { name: screen.heading, exact: true }).waitFor();
    if (screen.extra) await page.getByRole('button', { name: screen.extra, exact: true }).click();
    await page.waitForTimeout(300);
    expect(await page.evaluate(LAYOUT_CHECK)).toEqual([]);
    await expect(page).toHaveScreenshot(`${screen.name}.png`, { fullPage: true, animations: 'disabled', maxDiffPixelRatio: 0.01 });
  });
}
