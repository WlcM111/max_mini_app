import { cx } from '../lib/cx';

/** Прогресс онбординга: сегменты и подпись «Шаг N из M». */
export function Steps({ current, total }: { current: number; total: number }) {
  return (
    <div className="steps">
      <div className="steps__bar" aria-hidden="true">
        {Array.from({ length: total }, (_, index) => (
          <span key={index} className={cx('steps__seg', index < current - 1 && 'is-done', index === current - 1 && 'is-current')} />
        ))}
      </div>
      <span className="steps__label">
        Шаг {current} из {total}
      </span>
    </div>
  );
}
