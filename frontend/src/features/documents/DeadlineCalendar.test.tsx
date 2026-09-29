import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HttpResponse, http } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/msw/server';
import { DeadlineCalendar } from './DeadlineCalendar';
import { ORG_ID, documentItems } from '../../test/fixtures';

const base = 'http://localhost:3000/api/v1';

// «Сегодня» — 24.09.2026: фикстуры истекают 01.10.2026 (скоро) и 01.08.2026 (просрочен).
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(new Date('2026-09-24T09:00:00Z'));
});
afterEach(() => {
  vi.useRealTimers();
});

const renderCalendar = (props: { compact?: boolean; onPickDay?: (day: string) => void } = {}) =>
  renderWithProviders(<DeadlineCalendar organizationId={ORG_ID} timezone="Europe/Moscow" onOpen={() => undefined} {...props} />);

describe('T-FE-CAL: календарь сроков', () => {
  it('листает месяцы и показывает документы со сроком в выбранном месяце', async () => {
    const user = userEvent.setup();
    renderCalendar();
    expect(screen.getByRole('heading', { name: 'Сентябрь 2026' })).toBeInTheDocument();
    expect(screen.getByText('В сентябре сроков нет')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Следующий месяц' }));
    expect(screen.getByRole('heading', { name: 'Октябрь 2026' })).toBeInTheDocument();
    expect(await screen.findByText('В октябре истекает 1 документ')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '1 октября: 1 документ' })).toBeInTheDocument();
    expect(screen.getByText('1 октября, четверг')).toBeInTheDocument();
    expect(screen.getByText('Лицензия на алкоголь')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Сегодня' }));
    expect(screen.getByRole('heading', { name: 'Сентябрь 2026' })).toBeInTheDocument();
  });

  it('отмечает просроченный срок и фильтрует список по дню', async () => {
    const user = userEvent.setup();
    renderCalendar();
    await user.click(screen.getByRole('button', { name: 'Предыдущий месяц' }));
    const day = await screen.findByRole('button', { name: '1 августа: 1 документ' });
    expect(day).toHaveClass('calendar__cell--expired');
    await user.click(day);
    expect(day).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('Договор аренды')).toBeInTheDocument();
  });

  it('в компактном виде передаёт выбранный день наружу', async () => {
    const user = userEvent.setup();
    const onPickDay = vi.fn();
    renderCalendar({ compact: true, onPickDay });
    await user.click(screen.getByRole('button', { name: 'Следующий месяц' }));
    await user.click(await screen.findByRole('button', { name: '1 октября: 1 документ' }));
    expect(onPickDay).toHaveBeenCalledWith('2026-10-01');
  });

  it('собирает все страницы реестра по курсору', async () => {
    const cursors: (string | null)[] = [];
    server.use(
      http.get(`${base}/organizations/:organizationId/documents`, ({ request }) => {
        const cursor = new URL(request.url).searchParams.get('cursor');
        cursors.push(cursor);
        return cursor
          ? HttpResponse.json({ items: [documentItems[1]!], next_cursor: null })
          : HttpResponse.json({ items: [documentItems[0]!], next_cursor: 'page-2' });
      }),
    );
    const user = userEvent.setup();
    renderCalendar();
    await user.click(screen.getByRole('button', { name: 'Предыдущий месяц' }));
    expect(await screen.findByRole('button', { name: '1 августа: 1 документ' })).toBeInTheDocument();
    expect(cursors).toEqual([null, 'page-2']);
  });
});
