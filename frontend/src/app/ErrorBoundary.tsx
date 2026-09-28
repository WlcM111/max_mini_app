import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Button } from '../shared/ui/Button';
import { Icon } from '../shared/ui/Icon';
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
      <main className="system">
        <div className="system__card">
          <span className="system__glyph system__glyph--danger" aria-hidden="true">
            <Icon name="alert" size={34} />
          </span>
          <h1 className="system__title">Что-то пошло не так</h1>
          <p className="system__text">Перезапустите приложение — данные сохранены на сервере.</p>
          <div className="system__actions">
            <Button size="l" stretched icon="refresh" onClick={() => window.location.reload()}>
              Перезапустить
            </Button>
          </div>
        </div>
      </main>
    );
  }
}
