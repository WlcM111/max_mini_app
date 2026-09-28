import type { ReactNode } from 'react';
import { cx } from '../lib/cx';
import { Icon, type IconName } from './Icon';

/** Строка «подпись — значение»; пустые значения не показываются. */
export function KeyValue({ label, value, stacked = false }: { label: string; value: ReactNode; stacked?: boolean }) {
  if (value === null || value === undefined || value === '') return null;
  return (
    <div className={cx('kv', stacked && 'kv--stack')}>
      <span className="kv__label">{label}</span>
      <span className="kv__value">{value}</span>
    </div>
  );
}

interface NavRowProps {
  icon: IconName;
  title: string;
  hint?: ReactNode;
  danger?: boolean;
  chevron?: boolean;
  onClick: () => void;
}

/** Строка-переход с иконкой и шевроном. */
export function NavRow({ icon, title, hint, danger = false, chevron = true, onClick }: NavRowProps) {
  return (
    <button type="button" className={cx('nav-row', danger && 'nav-row--danger')} onClick={onClick}>
      <span className="nav-row__icon">
        <Icon name={icon} />
      </span>
      <span className="nav-row__text">
        <span className="nav-row__title">{title}</span>
        {hint ? <span className="nav-row__hint">{hint}</span> : null}
      </span>
      {chevron ? <Icon name="chevron-right" className="nav-row__chev" /> : null}
    </button>
  );
}
