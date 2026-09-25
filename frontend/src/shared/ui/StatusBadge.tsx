import type { DeadlineStatus } from '../../api/client';
import { daysLeftText } from '../lib/dates';

const TITLES: Record<DeadlineStatus, string> = {
  expired: 'Просрочен',
  expiring: 'Скоро истекает',
  valid: 'В порядке',
  no_expiry: 'Бессрочный',
};

interface Props {
  status: DeadlineStatus;
  daysLeft?: number | null;
  withDays?: boolean;
}

/** Статус срока: цвет и обязательно текст (доступность — не только цвет). */
export function StatusBadge({ status, daysLeft, withDays = true }: Props) {
  const text = TITLES[status];
  const days = withDays && status !== 'no_expiry' ? daysLeftText(daysLeft ?? null) : null;
  return (
    <span className={`badge badge--${status}`}>
      {text}
      {days ? <span className="muted" style={{ color: 'inherit' }}>· {days}</span> : null}
    </span>
  );
}
