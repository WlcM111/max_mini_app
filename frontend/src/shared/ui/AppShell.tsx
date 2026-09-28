import { useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { cx } from '../lib/cx';
import { getNavDirection } from '../lib/navDirection';
import { BackContext } from './backContext';
import { Icon } from './Icon';

interface Props {
  title: string;
  subtitle?: ReactNode;
  onTitleClick?: (() => void) | undefined;
  titleSkeleton?: boolean;
  onBack?: (() => void) | undefined;
  showHeaderBack?: boolean;
  banners?: ReactNode;
  headerExtra?: ReactNode;
  actions?: ReactNode;
  actionsNote?: ReactNode;
  fab?: ReactNode;
  wide?: boolean;
  bare?: boolean;
  children: ReactNode;
}

/**
 * Каркас экрана: крупный заголовок, компактная панель при прокрутке,
 * закреплённая панель действий и плавающая кнопка. Появление экрана
 * анимируется по направлению перехода.
 */
export function AppShell({
  title,
  subtitle,
  onTitleClick,
  titleSkeleton = false,
  onBack,
  showHeaderBack = false,
  banners,
  headerExtra,
  actions,
  actionsNote,
  fab,
  wide = false,
  bare = false,
  children,
}: Props) {
  const pageRef = useRef<HTMLElement>(null);
  const [enter] = useState(getNavDirection);
  const [scrolled, setScrolled] = useState(false);
  const fallbackBack = useContext(BackContext);
  const back = showHeaderBack && onBack ? onBack : fallbackBack;

  // Фокус на заголовок при смене экрана — для экранного диктора.
  useEffect(() => {
    const heading = pageRef.current?.querySelector('h1');
    if (!heading) return;
    if (!heading.hasAttribute('tabindex')) heading.setAttribute('tabindex', '-1');
    heading.focus({ preventScroll: true });
  }, [title]);

  // Компактная панель появляется, когда крупный заголовок ушёл под край.
  useEffect(() => {
    const heading = pageRef.current?.querySelector('h1');
    if (!heading || typeof IntersectionObserver === 'undefined') return undefined;
    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry) setScrolled(!entry.isIntersecting && entry.boundingClientRect.top < 0);
      },
      { rootMargin: '-48px 0px 0px 0px' },
    );
    observer.observe(heading);
    return () => observer.disconnect();
  }, [title, bare, titleSkeleton]);

  return (
    <div
      className={cx('shell', wide && 'shell--wide', fab ? 'shell--fab' : null, back ? 'shell--backbar' : null)}
      data-scrolled={scrolled ? 'true' : 'false'}
    >
      <div className="topbar" data-layer="top">
        {back ? (
          <button type="button" className="icon-btn topbar__back" aria-label="Назад" onClick={back}>
            <Icon name="chevron-left" size={26} />
          </button>
        ) : null}
        <span className="topbar__title" aria-hidden="true">
          {title}
        </span>
      </div>
      <main className="page" ref={pageRef} data-enter={enter}>
        {bare ? null : (
          <header className="page-head">
            {titleSkeleton ? (
              <>
                <h1 className="visually-hidden" tabIndex={-1}>
                  {title}
                </h1>
                <div className="skeleton skel-title" aria-hidden="true" />
              </>
            ) : (
              <h1 className="page-title" tabIndex={-1}>
                {onTitleClick ? (
                  <button type="button" className="title-switch" onClick={onTitleClick}>
                    <span>{title}</span>
                    <Icon name="chevrons" className="title-switch__icon" />
                  </button>
                ) : (
                  title
                )}
              </h1>
            )}
            {subtitle ? <p className="page-subtitle">{subtitle}</p> : null}
            {headerExtra}
          </header>
        )}
        {banners ? <div className="banners">{banners}</div> : null}
        {children}
      </main>
      {fab}
      {actions ? (
        <footer className="actionbar" data-layer="bottom">
          <div className="actionbar__inner">
            {actionsNote}
            {actions}
          </div>
        </footer>
      ) : null}
    </div>
  );
}
