import { useEffect, useRef, type ReactNode } from 'react';
import { Button } from '@maxhub/max-ui';

interface Props {
  title: string;
  subtitle?: string | undefined;
  onBack?: (() => void) | undefined;
  showHeaderBack?: boolean;
  banners?: ReactNode;
  actions?: ReactNode;
  headerExtra?: ReactNode;
  children: ReactNode;
}

/**
 * Каркас экрана: шапка с заголовком, прокручиваемое содержимое и закреплённая
 * панель действий. При смене экрана фокус переводится на заголовок.
 */
export function AppShell({ title, subtitle, onBack, showHeaderBack, banners, actions, headerExtra, children }: Props) {
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    headingRef.current?.focus();
  }, [title]);

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-header__row">
          {showHeaderBack && onBack ? (
            <Button size="small" variant="ghost" onClick={onBack} aria-label="Назад">
              ‹
            </Button>
          ) : null}
          <div className="grow">
            <h1 className="app-header__title" tabIndex={-1} ref={headingRef}>
              {title}
            </h1>
            {subtitle ? <p className="app-header__subtitle truncate">{subtitle}</p> : null}
          </div>
          {headerExtra}
        </div>
      </header>
      {banners ? <div style={{ padding: '10px 16px 0' }}>{banners}</div> : null}
      <main className="app-content">{children}</main>
      {actions ? <footer className="app-footer">{actions}</footer> : null}
    </div>
  );
}
