import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cx } from '../lib/cx';
import { Icon, type IconName } from './Icon';

type Variant = 'primary' | 'secondary' | 'neutral' | 'tertiary' | 'danger' | 'danger-soft';

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: 'l' | 'm' | 's';
  stretched?: boolean;
  loading?: boolean;
  icon?: IconName | undefined;
  iconRight?: IconName | undefined;
}

/** Кнопка: при загрузке показывает индикатор, но сохраняет подпись (доступное имя не меняется). */
export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { variant = 'primary', size = 'm', stretched = false, loading = false, icon, iconRight, className, children, disabled, type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={cx('btn', `btn--${variant}`, `btn--${size}`, stretched && 'btn--stretched', loading && 'is-loading', className)}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <span className="btn__spinner" aria-hidden="true" /> : icon ? <Icon name={icon} /> : null}
      <span className="btn__label">{children}</span>
      {iconRight && !loading ? <Icon name={iconRight} /> : null}
    </button>
  );
});
