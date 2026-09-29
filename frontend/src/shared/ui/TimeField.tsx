import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { cx } from '../lib/cx';
import { describedBy, FieldFrame } from './Field';
import { Icon } from './Icon';
import { Sheet } from './Sheet';

interface Props {
  id?: string;
  label: string;
  value: string;
  /** Допустимые значения HH:MM по возрастанию. */
  options: readonly string[];
  onChange: (value: string) => void;
  hint?: ReactNode;
  disabled?: boolean;
  pickerDescription?: string;
}

const GROUPS = [
  { title: 'Утро', from: 0, to: 12 },
  { title: 'День', from: 12, to: 18 },
  { title: 'Вечер', from: 18, to: 24 },
];

/**
 * Выбор времени сеткой значений в шторке: системный выпадающий список в тёмной
 * теме MAX для компьютера рисуется светлым и нечитаем.
 */
export function TimeField({ id: idProp, label, value, options, onChange, hint, disabled, pickerDescription }: Props) {
  const autoId = useId();
  const id = idProp ?? autoId;
  const trigger = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);

  const close = useCallback(() => {
    setOpen(false);
    trigger.current?.focus({ preventScroll: true });
  }, []);

  return (
    <FieldFrame id={id} label={label} hint={hint}>
      <div className={cx('control', 'control--picker', open && 'is-open')} data-disabled={disabled ? 'true' : undefined}>
        <Icon name="clock" className="control__icon" />
        <input
          ref={trigger}
          id={id}
          className="control__input control__input--time"
          type="text"
          readOnly
          value={value}
          disabled={disabled}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-describedby={describedBy(id, null, hint)}
          onClick={() => !disabled && setOpen(true)}
          onKeyDown={(event) => {
            if (!disabled && (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown')) {
              event.preventDefault();
              setOpen(true);
            }
          }}
        />
        <span className="control__trail" aria-hidden="true">
          <Icon name="chevron-down" size={18} />
        </span>
      </div>
      <Sheet open={open} onClose={close} title={label} description={pickerDescription}>
        <TimeOptions
          options={options}
          value={value}
          onPick={(option) => {
            if (option !== value) onChange(option);
            close();
          }}
        />
      </Sheet>
    </FieldFrame>
  );
}

/** Значения по частям суток; выбранное время сразу в фокусе и в поле зрения. */
function TimeOptions({ options, value, onPick }: { options: readonly string[]; value: string; onPick: (value: string) => void }) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const current = root.current?.querySelector<HTMLButtonElement>('[aria-pressed="true"]');
    (current ?? root.current?.querySelector<HTMLButtonElement>('button'))?.focus();
  }, []);
  const groups = GROUPS.map((group) => ({
    ...group,
    items: options.filter((option) => {
      const hour = Number(option.slice(0, 2));
      return hour >= group.from && hour < group.to;
    }),
  })).filter((group) => group.items.length > 0);
  return (
    <div className="time-groups" ref={root}>
      {groups.map((group) => (
        <div key={group.title} className="time-group" role="group" aria-label={group.title}>
          <span className="time-group__title" aria-hidden="true">
            {group.title}
          </span>
          <div className="time-grid">
            {group.items.map((option) => (
              <button key={option} type="button" className="time-chip" aria-pressed={option === value} onClick={() => onPick(option)}>
                {option}
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
