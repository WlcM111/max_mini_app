import type { DeadlineStatus } from '../../api/client';
import { cx } from '../lib/cx';
import { leafParts } from '../lib/format';
import { Icon } from './Icon';

interface Props {
  date: string | null | undefined;
  status: DeadlineStatus;
  size?: 'm' | 'l';
}

/** Листок отрывного календаря: месяц на цветной полосе статуса и число. */
export function DateLeaf({ date, status, size = 'm' }: Props) {
  const parts = date ? leafParts(date) : null;
  const showYear = parts !== null && Number(parts.year) !== new Date().getFullYear();
  return (
    <span className={cx('leaf', `leaf--${status}`, size === 'l' && 'leaf--l', showYear && 'leaf--year')} aria-hidden="true">
      <span className="leaf__band">{parts ? parts.month : ''}</span>
      {parts ? (
        <>
          <span className="leaf__day">{parts.day}</span>
          {showYear ? <span className="leaf__year">{parts.year}</span> : null}
        </>
      ) : (
        <span className="leaf__inf">
          <Icon name="infinity" size={size === 'l' ? 36 : 22} />
        </span>
      )}
    </span>
  );
}
