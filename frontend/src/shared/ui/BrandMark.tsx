import { cx } from '../lib/cx';

/** Знак «Вовремя»: стрелка продления по кругу и галочка (вариант 10). */
export function BrandMark({ size = 48, animated = false }: { size?: number; animated?: boolean }) {
  return (
    <span className={cx('brand-mark', animated && 'brand-mark--spin')} aria-hidden="true">
      <svg width={size} height={size} viewBox="0 0 120 120" focusable="false">
        <rect width="120" height="120" rx="30" fill="#1F8F59" />
        <g className="brand-mark__loop">
          <path d="M88.1 73.1A31 31 0 1 1 85.4 42.2" fill="none" stroke="#fff" strokeWidth="8" strokeLinecap="round" />
          <path d="M89.4 35.7L90.0 48.8L77.9 43.8z" fill="#fff" stroke="#fff" strokeWidth="3" strokeLinejoin="round" />
        </g>
        <path d="M48.5 60.5l8 8 15.5-16.5" fill="none" stroke="#fff" strokeWidth="7" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </span>
  );
}
