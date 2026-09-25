import { Button } from '@maxhub/max-ui';

interface Props {
  message: string;
  onRetry?: () => void;
  retryAfterSeconds?: number | undefined;
}

/** Ошибка запуска: устаревшие данные MAX, отказ сети или занятый сервис. */
export function LaunchErrorPage({ message, onRetry, retryAfterSeconds }: Props) {
  return (
    <div className="app-shell">
      <main className="app-content">
        <h1 className="app-header__title">Не удалось начать работу</h1>
        <p className="card__text" role="alert">
          {message}
        </p>
        {retryAfterSeconds ? <p className="muted">Повторите через {retryAfterSeconds} с.</p> : null}
        {onRetry ? (
          <Button size="large" stretched onClick={onRetry}>
            Повторить
          </Button>
        ) : null}
      </main>
    </div>
  );
}
