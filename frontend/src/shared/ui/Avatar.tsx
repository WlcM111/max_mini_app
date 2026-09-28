import { cx } from '../lib/cx';
import { toneIndex } from '../lib/format';

interface Props {
  text: string;
  seed: string;
  size?: 'm' | 'l';
  square?: boolean;
}

/** Круглая (или квадратная для организаций) плашка с инициалами. */
export function Avatar({ text, seed, size = 'm', square = false }: Props) {
  return (
    <span className={cx('avatar', size === 'l' && 'avatar--l', square && 'avatar--square', `avatar--t${toneIndex(seed)}`)} aria-hidden="true">
      {text}
    </span>
  );
}
