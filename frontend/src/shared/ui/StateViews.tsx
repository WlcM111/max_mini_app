import type { ReactNode } from 'react';
import { Button } from '@maxhub/max-ui';
import { messageForError } from '../../api/errors';

/** Состояние загрузки: скелетоны вместо пустого экрана. */
export function LoadingView({ rows = 3, label = 'Загрузка' }: { rows?: number; label?: string }) {
  return (
    <div className="stack" role="status" aria-live="polite" aria-busy="true">
      <span className="visually-hidden">{label}</span>
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} className="skeleton skeleton--row" />
      ))}
    </div>
  );
}

/** Пустое состояние с понятным следующим действием. */
export function EmptyView({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="state-view">
      <h2 className="state-view__title">{title}</h2>
      {description ? <p className="card__text">{description}</p> : null}
      {action}
    </div>
  );
}

/** Ошибка с кнопкой повтора; текст выбирается по коду ответа API. */
export function ErrorView({ error, onRetry, title }: { error: unknown; onRetry?: () => void; title?: string }) {
  return (
    <div className="state-view" role="alert" aria-live="polite">
      <h2 className="state-view__title">{title ?? 'Не удалось загрузить данные'}</h2>
      <p className="card__text">{messageForError(error)}</p>
      {onRetry ? (
        <Button size="medium" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      ) : null}
    </div>
  );
}
