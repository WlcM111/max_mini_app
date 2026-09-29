import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { cx } from '../lib/cx';
import { addDays, dayLabel, MONTHS, MONTHS_SHORT, monthCells, shiftMonth, WEEKDAYS } from '../lib/calendar';
import { Icon } from './Icon';

interface Props {
  /** Показанный месяц YYYY-MM. */
  month: string;
  onMonthChange: (month: string) => void;
  /** Выделенная дата YYYY-MM-DD. */
  selected: string | null;
  onSelect: (value: string) => void;
  today: string;
  min?: string | undefined;
  /** Перевести фокус на выбранный день при появлении (открытие шторки). */
  autoFocus?: boolean;
}

// Сдвиг фокуса клавишами: как в сетке календаря WAI-ARIA.
const KEY_STEPS: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };

/** Выбор даты в стиле календаря сроков: месяц листками, быстрый переход по месяцам и годам. */
export function DatePicker({ month, onMonthChange, selected, onSelect, today, min, autoFocus = false }: Props) {
  const [view, setView] = useState<'days' | 'months'>('days');
  const [direction, setDirection] = useState<'next' | 'prev' | 'none'>('none');
  const [focused, setFocused] = useState(() => {
    const start = selected ?? today;
    return min && start < min ? min : start;
  });
  const grid = useRef<HTMLDivElement>(null);
  const moveFocus = useRef(autoFocus);
  const [year = 2026, monthNumber = 1] = month.split('-').map(Number);
  const minMonth = min?.slice(0, 7);
  const cells = monthCells(month);
  // День, на который попадает Tab: фокус, выбранная дата или первый доступный день месяца.
  const anchor =
    focused.startsWith(month) ? focused : selected?.startsWith(month) ? selected : cells.find((day) => day !== null && (!min || day >= min));

  // Фокус переходит на день только после действий с клавиатуры или при открытии.
  useEffect(() => {
    if (!moveFocus.current || view !== 'days') return;
    moveFocus.current = false;
    grid.current?.querySelector<HTMLButtonElement>(`[data-day="${focused}"]`)?.focus();
  }, [focused, month, view]);

  const go = (delta: number) => {
    setDirection(delta > 0 ? 'next' : 'prev');
    onMonthChange(shiftMonth(month, delta));
  };

  const onGridKey = (event: KeyboardEvent<HTMLDivElement>) => {
    let next: string | null = null;
    if (event.key in KEY_STEPS) next = addDays(focused, KEY_STEPS[event.key] ?? 0);
    else if (event.key === 'PageUp') next = `${shiftMonth(focused.slice(0, 7), event.shiftKey ? -12 : -1)}-${focused.slice(8)}`;
    else if (event.key === 'PageDown') next = `${shiftMonth(focused.slice(0, 7), event.shiftKey ? 12 : 1)}-${focused.slice(8)}`;
    if (!next) return;
    event.preventDefault();
    // 31 марта → PageDown: такого дня нет, берётся последний день месяца.
    const days = monthCells(next.slice(0, 7)).filter((day): day is string => day !== null);
    const target = days.includes(next) ? next : (days[days.length - 1] ?? next);
    if (min && target < min) return;
    if (target.slice(0, 7) !== month) {
      setDirection(target > focused ? 'next' : 'prev');
      onMonthChange(target.slice(0, 7));
    }
    moveFocus.current = true;
    setFocused(target);
  };

  if (view === 'months') {
    return (
      <div className="picker">
        <div className="calendar__head">
          <button type="button" className="icon-btn" aria-label="Предыдущий год" onClick={() => onMonthChange(`${year - 1}-${month.slice(5)}`)}>
            <Icon name="chevron-left" size={22} />
          </button>
          <span className="calendar__title" aria-live="polite">
            {year}
          </span>
          <button type="button" className="icon-btn" aria-label="Следующий год" onClick={() => onMonthChange(`${year + 1}-${month.slice(5)}`)}>
            <Icon name="chevron-right" size={22} />
          </button>
        </div>
        <div className="picker__months" role="group" aria-label={`Месяцы ${year} года`}>
          {MONTHS_SHORT.map((name, index) => {
            const value = `${year}-${String(index + 1).padStart(2, '0')}`;
            return (
              <button
                key={value}
                type="button"
                className={cx('picker__month', value === today.slice(0, 7) && 'is-current')}
                aria-pressed={selected?.slice(0, 7) === value}
                aria-label={`${MONTHS[index]} ${year}`}
                disabled={minMonth !== undefined && value < minMonth}
                onClick={() => {
                  setDirection('none');
                  onMonthChange(value);
                  setView('days');
                }}
              >
                {name}
              </button>
            );
          })}
        </div>
      </div>
    );
  }

  return (
    <div className="picker">
      <div className="calendar__head">
        <button
          type="button"
          className="icon-btn"
          aria-label="Предыдущий месяц"
          disabled={minMonth !== undefined && month <= minMonth}
          onClick={() => go(-1)}
        >
          <Icon name="chevron-left" size={22} />
        </button>
        <button type="button" className="picker__title" aria-label={`${MONTHS[monthNumber - 1] ?? ''} ${year}: выбрать месяц и год`} onClick={() => setView('months')}>
          <span aria-live="polite">
            {MONTHS[monthNumber - 1]} {year}
          </span>
          <Icon name="chevron-down" size={18} />
        </button>
        <button type="button" className="icon-btn" aria-label="Следующий месяц" onClick={() => go(1)}>
          <Icon name="chevron-right" size={22} />
        </button>
      </div>
      <div className="calendar__weekdays" aria-hidden="true">
        {WEEKDAYS.map((day) => (
          <span key={day}>{day}</span>
        ))}
      </div>
      <div key={month} ref={grid} className="calendar__grid" data-dir={direction} onKeyDown={onGridKey}>
        {cells.map((day, index) => {
          if (!day) return <span key={`gap-${index}`} className="calendar__cell calendar__cell--gap" aria-hidden="true" />;
          return (
            <button
              key={day}
              type="button"
              data-day={day}
              className={cx('calendar__cell', day === today && 'is-today')}
              aria-pressed={day === selected}
              aria-current={day === today ? 'date' : undefined}
              aria-label={dayLabel(day)}
              disabled={min !== undefined && day < min}
              tabIndex={day === anchor ? 0 : -1}
              onFocus={() => setFocused(day)}
              onClick={() => onSelect(day)}
            >
              <span className="calendar__day">{Number(day.slice(8))}</span>
            </button>
          );
        })}
      </div>
      <div className="calendar__foot">
        <span>{selected ? `Выбрано: ${dayLabel(selected).replace(/, [^,]+$/, '')}` : 'Выберите день'}</span>
        {month !== today.slice(0, 7) && (!min || today >= min) ? (
          <button
            type="button"
            className="calendar__today"
            onClick={() => {
              setDirection(month > today.slice(0, 7) ? 'prev' : 'next');
              onMonthChange(today.slice(0, 7));
              setFocused(today);
            }}
          >
            Сегодня
          </button>
        ) : null}
      </div>
    </div>
  );
}
