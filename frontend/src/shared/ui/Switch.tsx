import { useId, type ReactNode } from 'react';
import { cx } from '../lib/cx';

interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean | undefined;
  id?: string;
  label?: string;
}

export function Switch({ checked, onChange, disabled, id, label }: SwitchProps) {
  return (
    <span className="switch">
      <input
        id={id}
        type="checkbox"
        role="switch"
        className="switch__input"
        checked={checked}
        disabled={disabled}
        aria-label={label}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className="switch__track" aria-hidden="true">
        <span className="switch__thumb" />
      </span>
    </span>
  );
}

interface ToggleRowProps {
  title: ReactNode;
  hint?: ReactNode;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  flush?: boolean;
}

/** Строка с переключателем: нажимается целиком. */
export function ToggleRow({ title, hint, checked, onChange, disabled, flush }: ToggleRowProps) {
  const id = useId();
  return (
    <label className={cx('toggle-row', flush && 'toggle-row--flush')} htmlFor={id}>
      <span className="toggle-row__text">
        <span className="toggle-row__title">{title}</span>
        {hint ? <span className="toggle-row__hint">{hint}</span> : null}
      </span>
      <Switch id={id} checked={checked} onChange={onChange} disabled={disabled} />
    </label>
  );
}
