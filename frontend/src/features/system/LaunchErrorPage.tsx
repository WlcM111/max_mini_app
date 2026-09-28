import { useEffect, useState } from 'react';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';

interface Props {
  message: string;
  onRetry: () => void;
  retryAfterSeconds?: number | undefined;
}

/** Ошибка запуска: устаревшая сессия, недоступный сервис или сеть. */
export function LaunchErrorPage({ message, onRetry, retryAfterSeconds }: Props) {
  const expired = message.startsWith('Сессия запуска устарела');
  const [left, setLeft] = useState(retryAfterSeconds ?? 0);

  // Обратный отсчёт до разрешённого повтора (ответ 429/503 с Retry-After).
  useEffect(() => {
    if (left <= 0) return undefined;
    const timer = window.setTimeout(() => setLeft((value) => value - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [left]);

  return (
    <main className="system">
      <div className="system__card">
        <span className="system__glyph system__glyph--danger" aria-hidden="true">
          <Icon name={expired ? 'clock' : 'alert'} size={34} />
        </span>
        <h1 className="system__title" tabIndex={-1}>
          {expired ? 'Сессия запуска устарела' : 'Не удалось начать работу'}
        </h1>
        <p className="system__text">{expired ? 'Закройте мини-приложение и снова откройте его из чата с ботом.' : message}</p>
        <div className="system__actions">
          <Button variant={expired ? 'neutral' : 'primary'} size="l" stretched icon="refresh" disabled={left > 0} onClick={onRetry}>
            {left > 0 ? `Повторить через ${left} с` : 'Повторить'}
          </Button>
        </div>
      </div>
    </main>
  );
}
