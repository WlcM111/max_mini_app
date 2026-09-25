import { Button } from '@maxhub/max-ui';

/** Приложение открыто вне MAX: пользовательского контекста нет (spec §3). */
export function NotInMaxPage({ onRetry }: { onRetry?: () => void }) {
  return (
    <div className="app-shell">
      <main className="app-content">
        <h1 className="app-header__title">Откройте приложение из чата с ботом в MAX</h1>
        <p className="card__text">
          «Вовремя» работает внутри мессенджера MAX: так приложение узнаёт, кто вы, и показывает документы
          вашей организации.
        </p>
        {onRetry ? (
          <Button size="large" stretched variant="secondary" onClick={onRetry}>
            Повторить
          </Button>
        ) : null}
      </main>
    </div>
  );
}
