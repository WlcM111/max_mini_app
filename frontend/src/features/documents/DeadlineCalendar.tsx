import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { DeadlineStatus, DocumentListItem as Item } from '../../api/client';
import { cx } from '../../shared/lib/cx';
import { todayInTimeZone } from '../../shared/lib/dates';
import { plural } from '../../shared/lib/plural';
import { Icon } from '../../shared/ui/Icon';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { listDocuments } from './api';
import { DocumentListItem } from './DocumentListItem';

const MONTHS = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'];
const MONTHS_GEN = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'];
const MONTHS_PREP = ['январе', 'феврале', 'марте', 'апреле', 'мае', 'июне', 'июле', 'августе', 'сентябре', 'октябре', 'ноябре', 'декабре'];
const WEEKDAYS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
const WEEKDAY_NAMES = ['воскресенье', 'понедельник', 'вторник', 'среда', 'четверг', 'пятница', 'суббота'];
const RANK: Record<DeadlineStatus, number> = { expired: 3, expiring: 2, valid: 1, no_expiry: 0 };
const DOCS: [string, string, string] = ['документ', 'документа', 'документов'];

// Все документы организации (до 2000) — для раскладки по дням.
async function fetchAll(organizationId: string): Promise<Item[]> {
  const items: Item[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < 20; page += 1) {
    const result = await listDocuments(organizationId, { limit: 100, ...(cursor ? { cursor } : {}) });
    items.push(...result.items);
    if (!result.next_cursor) break;
    cursor = result.next_cursor;
  }
  return items;
}

const shiftMonth = (month: string, delta: number) => {
  const [year = 2026, index = 1] = month.split('-').map(Number);
  const date = new Date(Date.UTC(year, index - 1 + delta, 1));
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
};

function dayTitle(day: string, today: string): string {
  const [year = 2026, month = 1, date = 1] = day.split('-').map(Number);
  const weekday = WEEKDAY_NAMES[new Date(Date.UTC(year, month - 1, date)).getUTCDay()] ?? '';
  const label = `${date} ${MONTHS_GEN[month - 1] ?? ''}`;
  return day === today ? `Сегодня, ${label}` : `${label}, ${weekday}`;
}

interface Props {
  organizationId: string;
  timezone: string;
  onOpen: (id: string) => void;
}

/** Календарь сроков: месяц листками, под ним — документы выбранного дня или месяца. */
export function DeadlineCalendar({ organizationId, timezone, onOpen }: Props) {
  const today = todayInTimeZone(timezone);
  const [month, setMonth] = useState(today.slice(0, 7));
  const [direction, setDirection] = useState<'next' | 'prev' | 'none'>('none');
  const [selected, setSelected] = useState<string | null>(null);
  const documents = useQuery({
    queryKey: ['documents', organizationId, 'calendar'],
    queryFn: () => fetchAll(organizationId),
    enabled: organizationId !== '',
  });

  const byDay = useMemo(() => {
    const map = new Map<string, Item[]>();
    for (const item of documents.data ?? []) {
      if (!item.valid_until) continue;
      map.set(item.valid_until, [...(map.get(item.valid_until) ?? []), item]);
    }
    return map;
  }, [documents.data]);

  const [year = 2026, monthNumber = 1] = month.split('-').map(Number);
  const lead = (new Date(Date.UTC(year, monthNumber - 1, 1)).getUTCDay() + 6) % 7;
  const length = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate();
  const cells: (string | null)[] = [
    ...Array.from({ length: lead }, () => null),
    ...Array.from({ length }, (_, index) => `${month}-${String(index + 1).padStart(2, '0')}`),
  ];
  while (cells.length % 7 !== 0) cells.push(null);

  const monthDays = [...byDay.entries()].filter(([day]) => day.startsWith(month)).sort(([a], [b]) => a.localeCompare(b));
  const monthCount = monthDays.reduce((sum, [, list]) => sum + list.length, 0);
  const shown = selected ? monthDays.filter(([day]) => day === selected) : monthDays;

  const go = (delta: number) => {
    setMonth((current) => shiftMonth(current, delta));
    setDirection(delta > 0 ? 'next' : 'prev');
    setSelected(null);
  };

  return (
    <section className="calendar" aria-labelledby="calendar-title">
      <div className="calendar__panel">
        <div className="calendar__head">
          <button type="button" className="icon-btn" aria-label="Предыдущий месяц" onClick={() => go(-1)}>
            <Icon name="chevron-left" size={22} />
          </button>
          <h2 className="calendar__title" id="calendar-title" aria-live="polite">
            {MONTHS[monthNumber - 1]} {year}
          </h2>
          <button type="button" className="icon-btn" aria-label="Следующий месяц" onClick={() => go(1)}>
            <Icon name="chevron-right" size={22} />
          </button>
        </div>
        <div className="calendar__weekdays" aria-hidden="true">
          {WEEKDAYS.map((day) => (
            <span key={day}>{day}</span>
          ))}
        </div>
        <div key={month} className="calendar__grid" data-dir={direction}>
          {cells.map((day, index) => {
            if (!day) return <span key={`gap-${index}`} className="calendar__cell calendar__cell--gap" aria-hidden="true" />;
            const items = byDay.get(day) ?? [];
            const number = Number(day.slice(8));
            const state = cx('calendar__cell', day === today && 'is-today', day < today && 'is-past');
            if (items.length === 0) {
              return (
                <span key={day} className={state}>
                  <span className="calendar__day">{number}</span>
                </span>
              );
            }
            const worst = items.reduce<DeadlineStatus>((acc, item) => (RANK[item.status] > RANK[acc] ? item.status : acc), 'no_expiry');
            return (
              <button
                key={day}
                type="button"
                className={cx(state, `calendar__cell--${worst}`)}
                aria-pressed={selected === day}
                aria-label={`${number} ${MONTHS_GEN[monthNumber - 1] ?? ''}: ${items.length} ${plural(items.length, DOCS)}`}
                onClick={() => setSelected((current) => (current === day ? null : day))}
              >
                <span className="calendar__day">{number}</span>
                <span className="calendar__mark" aria-hidden="true">
                  {items.length > 1 ? items.length : ''}
                </span>
              </button>
            );
          })}
        </div>
        <div className="calendar__foot">
          <span>
            {monthCount > 0
              ? `В ${MONTHS_PREP[monthNumber - 1] ?? ''} истекает ${monthCount} ${plural(monthCount, DOCS)}`
              : `В ${MONTHS_PREP[monthNumber - 1] ?? ''} сроков нет`}
          </span>
          {month !== today.slice(0, 7) ? (
            <button
              type="button"
              className="calendar__today"
              onClick={() => {
                setDirection(month > today.slice(0, 7) ? 'prev' : 'next');
                setMonth(today.slice(0, 7));
                setSelected(null);
              }}
            >
              Сегодня
            </button>
          ) : null}
        </div>
      </div>

      <div className="calendar__list">
        {documents.isLoading ? <LoadingView rows={3} /> : null}
        {documents.isError ? <ErrorView error={documents.error} onRetry={() => void documents.refetch()} /> : null}
        {shown.map(([day, list]) => (
          <div key={day} className="calendar__group">
            <h3 className="calendar__date">{dayTitle(day, today)}</h3>
            <div className="list stagger">
              {list.map((item) => (
                <DocumentListItem key={item.id} item={item} onOpen={onOpen} />
              ))}
            </div>
          </div>
        ))}
        {documents.isSuccess && shown.length === 0 ? (
          <p className="calendar__empty">Отмеченные дни — это даты окончания сроков. Листайте месяцы стрелками.</p>
        ) : null}
      </div>
    </section>
  );
}
