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

/** Статус срока: цвет и текст, чтобы статус не передавался одним цветом. */
export function StatusBadge({ status, daysLeft, withDays = true }: Props) {
  const days = withDays && status !== 'no_expiry' ? daysLeftText(daysLeft) : '';
  return (
    <span className={`status status--${status}`}>
      {TITLES[status]}
      {days ? <span className="status__days">{days}</span> : null}
    </span>
  );
}
