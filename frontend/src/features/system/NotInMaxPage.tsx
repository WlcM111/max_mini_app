import { BrandMark } from '../../shared/ui/BrandMark';
import { Button } from '../../shared/ui/Button';

/** Приложение открыто вне MAX: без данных запуска работа невозможна. */
export function NotInMaxPage({ onRetry }: { onRetry?: () => void }) {
  return (
    <main className="system">
      <div className="system__card">
        <BrandMark size={72} />
        <h1 className="system__title" tabIndex={-1}>
          Откройте приложение из чата с ботом в MAX
        </h1>
        <p className="system__text">
          «Вовремя» работает внутри мессенджера MAX: так приложение узнаёт, кто вы, и показывает документы вашей организации.
        </p>
        {onRetry ? (
          <div className="system__actions">
            <Button variant="secondary" size="l" stretched icon="refresh" onClick={onRetry}>
              Повторить
            </Button>
          </div>
        ) : null}
      </div>
    </main>
  );
}
