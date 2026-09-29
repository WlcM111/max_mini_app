import { useCallback, useId, useRef, useState } from 'react';
import { cx } from '../lib/cx';
import { formatDate } from '../lib/dates';
import { localToday, maskTypedDate, parseTypedDate } from '../lib/calendar';
import { Button } from './Button';
import { DatePicker } from './DatePicker';
import { describedBy, FieldFrame } from './Field';
import { Icon } from './Icon';
import { Sheet } from './Sheet';

interface Props {
  id?: string;
  label: string;
  value: string | null;
  onChange: (value: string | null) => void;
  error?: string | undefined;
  hint?: string;
  disabled?: boolean;
  min?: string | undefined;
  flash?: boolean;
  placeholder?: string;
  /** Пояснение в шторке выбора, например название документа. */
  pickerDescription?: string | undefined;
}

/**
 * Поле даты: открывает календарь приложения в шторке (на компьютере — диалог),
 * а не системный выбор даты. Дату можно и ввести вручную в формате ДД.ММ.ГГГГ.
 */
export function DateField({
  id: idProp,
  label,
  value,
  onChange,
  error,
  hint,
  disabled,
  min,
  flash,
  placeholder = 'Выберите дату',
  pickerDescription,
}: Props) {
  const autoId = useId();
  const id = idProp ?? autoId;
  const typedId = `${id}-typed`;
  const trigger = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [today, setToday] = useState(localToday);
  const [month, setMonth] = useState('');
  const [typed, setTyped] = useState('');
  const [typedError, setTypedError] = useState<string | null>(null);
  const typedValue = parseTypedDate(typed);

  const show = () => {
    if (disabled) return;
    const now = localToday();
    setToday(now);
    setMonth((value ?? (min && now < min ? min : now)).slice(0, 7));
    setTyped(value ? formatDate(value) : '');
    setTypedError(null);
    setOpen(true);
  };
  const close = useCallback(() => {
    setOpen(false);
    trigger.current?.focus({ preventScroll: true });
  }, []);
  const commit = (next: string | null) => {
    onChange(next);
    close();
  };
  const applyTyped = () => {
    if (!typedValue) {
      setTypedError('Введите дату в формате ДД.ММ.ГГГГ');
      return;
    }
    if (min && typedValue < min) {
      setTypedError(`Не раньше ${formatDate(min)}`);
      return;
    }
    commit(typedValue);
  };

  return (
    <FieldFrame id={id} label={label} hint={hint} error={error}>
      <div
        className={cx('control', 'control--picker', flash && 'control--flash', open && 'is-open')}
        data-invalid={error ? 'true' : undefined}
        data-disabled={disabled ? 'true' : undefined}
      >
        <Icon name="calendar" className="control__icon" />
        <input
          ref={trigger}
          id={id}
          className="control__input"
          type="text"
          readOnly
          value={value ? formatDate(value) : ''}
          placeholder={placeholder}
          disabled={disabled}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(id, error, hint)}
          onClick={show}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown') {
              event.preventDefault();
              show();
            }
          }}
        />
        {value && !disabled ? (
          <button type="button" className="control__action" aria-label="Очистить дату" onClick={() => onChange(null)}>
            <Icon name="x" size={18} />
          </button>
        ) : (
          <span className="control__trail" aria-hidden="true">
            <Icon name="chevron-down" size={18} />
          </span>
        )}
      </div>
      <Sheet open={open} onClose={close} title={label} description={pickerDescription}>
        <div className="picker-sheet">
          <div className="field">
            <label className="field__label" htmlFor={typedId}>
              Дата вручную
            </label>
            <div className="control" data-invalid={typedError ? 'true' : undefined}>
              <input
                id={typedId}
                className="control__input"
                type="text"
                inputMode="numeric"
                autoComplete="off"
                placeholder="ДД.ММ.ГГГГ"
                maxLength={10}
                value={typed}
                aria-invalid={typedError ? true : undefined}
                aria-describedby={typedError ? `${typedId}-error` : undefined}
                onChange={(event) => {
                  const next = maskTypedDate(event.target.value);
                  setTyped(next);
                  setTypedError(null);
                  const parsed = parseTypedDate(next);
                  if (parsed) setMonth(parsed.slice(0, 7));
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    applyTyped();
                  }
                }}
              />
              <button
                type="button"
                className="control__action control__action--accent"
                aria-label="Применить введённую дату"
                disabled={!typedValue}
                onClick={applyTyped}
              >
                <Icon name="check" size={20} />
              </button>
            </div>
            {typedError ? (
              <span className="field__error" id={`${typedId}-error`} role="alert">
                <Icon name="alert" size={16} />
                {typedError}
              </span>
            ) : null}
          </div>
          <DatePicker
            month={month}
            onMonthChange={setMonth}
            selected={typedValue ?? value}
            onSelect={commit}
            today={today}
            min={min}
            autoFocus
          />
          <div className="picker-sheet__actions">
            {value ? (
              <Button variant="tertiary" icon="x" onClick={() => commit(null)}>
                Очистить
              </Button>
            ) : null}
            <Button variant="neutral" onClick={close}>
              Отмена
            </Button>
          </div>
        </div>
      </Sheet>
    </FieldFrame>
  );
}
