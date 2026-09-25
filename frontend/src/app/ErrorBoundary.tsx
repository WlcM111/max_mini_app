import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Button } from '@maxhub/max-ui';
import { track } from '../shared/lib/telemetry';

interface Props {
  children: ReactNode;
}

interface State {
  failed: boolean;
}

/** Последний рубеж: непойманная ошибка рендера не оставляет пустой экран. */
export class ErrorBoundary extends Component<Props, State> {
  override state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  override componentDidCatch(_error: Error, _info: ErrorInfo): void {
    track({ name: 'api_error_shown', code: 'render' });
  }

  override render(): ReactNode {
    if (!this.state.failed) return this.props.children;
    return (
      <div className="app-shell">
        <main className="app-content">
          <h1 className="app-header__title">Что-то пошло не так</h1>
          <p className="card__text">Перезапустите приложение — данные сохранены на сервере.</p>
          <Button size="large" stretched onClick={() => window.location.reload()}>
            Перезапустить
          </Button>
        </main>
      </div>
    );
  }
}
