import type { ReactNode } from 'react';
import { cx } from '../lib/cx';
import { Icon, type IconName } from './Icon';

interface Props {
  tone?: 'info' | 'warning' | 'danger' | 'success';
  icon?: IconName;
  title?: ReactNode;
  children?: ReactNode;
  className?: string;
}

export function Banner({ tone = 'info', icon, title, children, className }: Props) {
  return (
    <div className={cx('banner', `banner--${tone}`, className)}>
      {icon ? <Icon name={icon} size={22} className="banner__icon" /> : null}
      <div className="banner__body">
        {title ? <strong className="banner__title">{title}</strong> : null}
        {children}
      </div>
    </div>
  );
}
