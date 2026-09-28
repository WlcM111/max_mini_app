import { useId, useState } from 'react';
import { cx } from '../lib/cx';
import { DAY_FORMS } from '../lib/format';
import { pluralWithCount } from '../lib/plural';
import { MAX_OFFSETS, OFFSET_PRESETS } from '../lib/validation';
import { Button } from './Button';
import { Icon } from './Icon';

interface Props {
  value: number[];
  onChange: (value: number[]) => void;
  error?: string | undefined;
}

const offsetLabel = (days: number) => (days === 0 ? 'в день срока' : `за ${pluralWithCount(days, DAY_FORMS)}`);

/** Выбор до пяти отступов напоминаний и ввод своего значения (0…365 дней). */
export function OffsetChips({ value, onChange, error }: Props) {
  const id = useId();
  const [custom, setCustom] = useState('');
  const [customError, setCustomError] = useState<string | null>(null);

  const toggle = (days: number) => {
    if (value.includes(days)) {
      setCustomError(null);
      onChange(value.filter((item) => item !== days));
      return;
    }
    if (value.length >= MAX_OFFSETS) {
      setCustomError('Не более 5 напоминаний');
      return;
    }
    setCustomError(null);
    onChange([...value, days].sort((a, b) => b - a));
  };

  const addCustom = () => {
    const parsed = Number(custom);
    if (custom.trim() === '' || !Number.isInteger(parsed) || parsed < 0 || parsed > 365) {
      setCustomError('Напоминание — от 0 до 365 дней');
      return;
    }
    if (value.includes(parsed)) {
      setCustomError('Такое напоминание уже добавлено');
      return;
    }
    if (value.length >= MAX_OFFSETS) {
      setCustomError('Не более 5 напоминаний');
      return;
    }
    setCustomError(null);
    setCustom('');
    onChange([...value, parsed].sort((a, b) => b - a));
  };

  const extra = value.filter((days) => !(OFFSET_PRESETS as readonly number[]).includes(days));
  const message = error ?? customError;

  return (
    <div className="field offsets">
      <span className="field__label" id={`${id}-label`}>
        Напомнить заранее
      </span>
      <div className="chips" role="group" aria-labelledby={`${id}-label`}>
        {[...OFFSET_PRESETS, ...extra].map((days) => {
          const on = value.includes(days);
          return (
            <button key={days} type="button" className="chip" aria-pressed={on} onClick={() => toggle(days)}>
              <Icon name="check" size={16} className="chip__check" />
              {offsetLabel(days)}
            </button>
          );
        })}
      </div>
      <div className="offsets__custom">
        <div className="control">
          <input
            id={`${id}-custom`}
            className="control__input"
            inputMode="numeric"
            placeholder="Своё значение, дней"
            aria-label="Своё количество дней"
            value={custom}
            onChange={(event) => setCustom(event.target.value.replace(/[^0-9]/g, '').slice(0, 3))}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                addCustom();
              }
            }}
          />
        </div>
        <Button variant="neutral" onClick={addCustom}>
          Добавить
        </Button>
      </div>
      <div className="offsets__meter">
        <span className="field__hint">
          Выбрано {value.length} из {MAX_OFFSETS}
        </span>
        <span className="dots" aria-hidden="true">
          {Array.from({ length: MAX_OFFSETS }, (_, index) => (
            <span key={index} className={cx('dots__dot', index < value.length && 'is-on')} />
          ))}
        </span>
      </div>
      {message ? (
        <span className="field__error" role="alert" aria-live="polite">
          <Icon name="alert" size={16} />
          {message}
        </span>
      ) : null}
    </div>
  );
}
