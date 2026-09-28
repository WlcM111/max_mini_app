import { useId } from 'react';
import { cx } from '../lib/cx';
import { describedBy, FieldFrame } from './Field';
import { Icon } from './Icon';

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
}

/**
 * Поле даты на нативном календаре. Вся область поля открывает выбор даты,
 * пустое значение показывает подсказку (iOS не рисует маску сам).
 */
export function DateField({ id: idProp, label, value, onChange, error, hint, disabled, min, flash, placeholder = 'Выберите дату' }: Props) {
  const autoId = useId();
  const id = idProp ?? autoId;
  const empty = !value;
  return (
    <FieldFrame id={id} label={label} hint={hint} error={error}>
      <div
        className={cx('control', 'control--date', flash && 'control--flash')}
        data-empty={empty ? 'true' : 'false'}
        data-invalid={error ? 'true' : undefined}
        data-disabled={disabled ? 'true' : undefined}
      >
        <Icon name="calendar" className="control__icon" />
        <input
          id={id}
          className="control__input"
          type="date"
          value={value ?? ''}
          min={min}
          disabled={disabled}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(id, error, hint)}
          onChange={(event) => onChange(event.target.value === '' ? null : event.target.value)}
        />
        {empty ? (
          <span className="control__placeholder" aria-hidden="true">
            {placeholder}
          </span>
        ) : null}
        {!empty && !disabled ? (
          <button type="button" className="control__action" aria-label="Очистить дату" onClick={() => onChange(null)}>
            <Icon name="x" size={18} />
          </button>
        ) : null}
      </div>
    </FieldFrame>
  );
}
