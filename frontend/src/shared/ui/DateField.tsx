import { useId } from 'react';

interface Props {
  label: string;
  value: string | null;
  onChange: (value: string | null) => void;
  error?: string | undefined;
  hint?: string;
  disabled?: boolean;
  min?: string;
}

/** Ввод календарной даты YYYY-MM-DD нативным полем клиента. */
export function DateField({ label, value, onChange, error, hint, disabled, min }: Props) {
  const id = useId();
  return (
    <div className="field">
      <label className="field__label" htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        type="date"
        value={value ?? ''}
        min={min}
        disabled={disabled}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : hint ? `${id}-hint` : undefined}
        onChange={(event) => onChange(event.target.value === '' ? null : event.target.value)}
      />
      {hint && !error ? (
        <span className="field__hint" id={`${id}-hint`}>
          {hint}
        </span>
      ) : null}
      {error ? (
        <span className="field__error" id={`${id}-error`} role="alert" aria-live="polite">
          {error}
        </span>
      ) : null}
    </div>
  );
}
