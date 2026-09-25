import { useId, useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { MAX_OFFSETS, OFFSET_PRESETS } from '../lib/validation';
import { pluralWithCount } from '../lib/plural';

interface Props {
  value: number[];
  onChange: (value: number[]) => void;
  error?: string | undefined;
}

/** Выбор до пяти отступов напоминаний и ввод своего значения (0…365 дней). */
export function OffsetChips({ value, onChange, error }: Props) {
  const id = useId();
  const [custom, setCustom] = useState('');
  const [customError, setCustomError] = useState<string | null>(null);

  const toggle = (days: number) => {
    if (value.includes(days)) {
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
    if (!Number.isInteger(parsed) || parsed < 0 || parsed > 365) {
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

  return (
    <div className="field">
      <span className="field__label" id={`${id}-label`}>
        Напомнить заранее
      </span>
      <div className="chips" role="group" aria-labelledby={`${id}-label`}>
        {OFFSET_PRESETS.map((days) => (
          <button
            key={days}
            type="button"
            className="chip"
            aria-pressed={value.includes(days)}
            onClick={() => toggle(days)}
          >
            {days === 0 ? 'в день срока' : `за ${pluralWithCount(days, ['день', 'дня', 'дней'])}`}
          </button>
        ))}
        {value
          .filter((days) => !OFFSET_PRESETS.includes(days as (typeof OFFSET_PRESETS)[number]))
          .map((days) => (
            <button key={days} type="button" className="chip" aria-pressed onClick={() => toggle(days)}>
              за {pluralWithCount(days, ['день', 'дня', 'дней'])}
            </button>
          ))}
      </div>
      <div className="row">
        <input
          type="text"
          inputMode="numeric"
          className="grow"
          placeholder="Своё значение, дней"
          aria-label="Своё количество дней"
          value={custom}
          onChange={(event) => setCustom(event.target.value.replace(/[^\d]/g, ''))}
        />
        <Button size="medium" variant="secondary" onClick={addCustom} disabled={custom === ''}>
          Добавить
        </Button>
      </div>
      <span className="field__hint">Выбрано {value.length} из {MAX_OFFSETS}</span>
      {(error ?? customError) ? (
        <span className="field__error" role="alert" aria-live="polite">
          {error ?? customError}
        </span>
      ) : null}
    </div>
  );
}
