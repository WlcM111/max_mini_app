import { useId, type ReactNode } from 'react';
import { cx } from '../lib/cx';
import { Icon, type IconName } from './Icon';

interface BaseProps {
  id?: string | undefined;
  label: string;
  labelHidden?: boolean;
  hint?: ReactNode;
  error?: string | null | undefined;
  flash?: boolean;
  className?: string;
}

/** Подпись, поле и строка подсказки или ошибки. */
export function FieldFrame({ id, label, labelHidden, hint, error, className, children }: BaseProps & { id: string; children: ReactNode }) {
  return (
    <div className={cx('field', className)}>
      <label className={labelHidden ? 'visually-hidden' : 'field__label'} htmlFor={id}>
        {label}
      </label>
      {children}
      {error ? (
        <span className="field__error" id={`${id}-error`} role="alert">
          <Icon name="alert" size={16} />
          {error}
        </span>
      ) : hint ? (
        <span className="field__hint" id={`${id}-hint`}>
          {hint}
        </span>
      ) : null}
    </div>
  );
}

export const describedBy = (id: string, error: unknown, hint: unknown): string | undefined =>
  error ? `${id}-error` : hint ? `${id}-hint` : undefined;

interface TextFieldProps extends BaseProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: 'text' | 'url' | 'search';
  inputMode?: 'text' | 'url' | 'search' | 'numeric' | 'email' | 'tel';
  autoComplete?: string;
  maxLength?: number;
  icon?: IconName;
  clearable?: boolean;
  raised?: boolean;
  trailing?: ReactNode;
  disabled?: boolean;
}

export function TextField({
  id: idProp,
  label,
  labelHidden,
  hint,
  error,
  flash,
  className,
  value,
  onChange,
  placeholder,
  type = 'text',
  inputMode,
  autoComplete,
  maxLength,
  icon,
  clearable,
  raised,
  trailing,
  disabled,
}: TextFieldProps) {
  const autoId = useId();
  const id = idProp ?? autoId;
  return (
    <FieldFrame id={id} label={label} labelHidden={labelHidden} hint={hint} error={error} className={className}>
      <div
        className={cx('control', raised && 'control--raised', flash && 'control--flash')}
        data-invalid={error ? 'true' : undefined}
        data-disabled={disabled ? 'true' : undefined}
      >
        {icon ? <Icon name={icon} className="control__icon" /> : null}
        <input
          id={id}
          className="control__input"
          type={type}
          value={value}
          placeholder={placeholder}
          inputMode={inputMode}
          autoComplete={autoComplete ?? 'off'}
          maxLength={maxLength}
          disabled={disabled}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(id, error, hint)}
          onChange={(event) => onChange(event.target.value)}
        />
        {clearable && value !== '' ? (
          <button type="button" className="control__action" aria-label="Очистить поле" onClick={() => onChange('')}>
            <Icon name="x" size={18} />
          </button>
        ) : null}
        {trailing}
      </div>
    </FieldFrame>
  );
}

interface TextAreaFieldProps extends BaseProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  rows?: number;
  maxLength?: number;
}

export function TextAreaField({ id: idProp, label, labelHidden, hint, error, flash, className, value, onChange, placeholder, rows = 3, maxLength }: TextAreaFieldProps) {
  const autoId = useId();
  const id = idProp ?? autoId;
  return (
    <FieldFrame id={id} label={label} labelHidden={labelHidden} hint={hint} error={error} className={className}>
      <div className={cx('control', flash && 'control--flash')} data-invalid={error ? 'true' : undefined}>
        <textarea
          id={id}
          className="control__input"
          value={value}
          rows={rows}
          placeholder={placeholder}
          maxLength={maxLength}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(id, error, hint)}
          onChange={(event) => onChange(event.target.value)}
        />
      </div>
    </FieldFrame>
  );
}

interface SelectFieldProps extends BaseProps {
  value: string;
  onChange: (value: string) => void;
  options: readonly { value: string; label: string }[];
  disabled?: boolean;
}

export function SelectField({ id: idProp, label, labelHidden, hint, error, flash, className, value, onChange, options, disabled }: SelectFieldProps) {
  const autoId = useId();
  const id = idProp ?? autoId;
  return (
    <FieldFrame id={id} label={label} labelHidden={labelHidden} hint={hint} error={error} className={className}>
      <div
        className={cx('control', flash && 'control--flash')}
        data-invalid={error ? 'true' : undefined}
        data-disabled={disabled ? 'true' : undefined}
      >
        <select
          id={id}
          className="control__input"
          value={value}
          disabled={disabled}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(id, error, hint)}
          onChange={(event) => onChange(event.target.value)}
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <Icon name="chevron-down" size={18} className="control__chevron" />
      </div>
    </FieldFrame>
  );
}
