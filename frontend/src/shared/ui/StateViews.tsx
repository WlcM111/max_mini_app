import type { ReactNode } from 'react';
import { messageForError, NetworkError } from '../../api/errors';
import { Button } from './Button';
import { DateLeaf } from './DateLeaf';
import { Icon, type IconName } from './Icon';

interface LoadingProps {
  rows?: number;
  label?: string;
  variant?: 'rows' | 'card' | 'form';
}

/** Скелетон: повторяет форму будущего содержимого, чтобы экран не «прыгал». */
export function LoadingView({ rows = 3, label = 'Загрузка', variant = 'rows' }: LoadingProps) {
  return (
    <div className="skeletons" role="status" aria-live="polite">
      <span className="visually-hidden">{label}</span>
      {variant === 'card' ? <div className="skeleton skel-block" /> : null}
      {variant === 'form'
        ? Array.from({ length: rows }, (_, index) => (
            <div key={index} className="skel-lines">
              <span className="skeleton skel-line skel-line--40" />
              <span className="skeleton skel-line" />
            </div>
          ))
        : Array.from({ length: rows }, (_, index) => (
            <div key={index} className="skel-row">
              <span className="skeleton skel-leaf" />
              <span className="skel-lines">
                <span className="skeleton skel-line" />
                <span className="skeleton skel-line skel-line--60" />
              </span>
            </div>
          ))}
    </div>
  );
}

type Art = 'leaves' | 'search' | 'people' | 'link' | 'none';
const GLYPHS: Record<Exclude<Art, 'leaves' | 'none'>, IconName> = { search: 'search', people: 'people', link: 'link' };

interface EmptyProps {
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  art?: Art;
}

export function EmptyView({ title, description, action, art = 'leaves' }: EmptyProps) {
  const year = new Date().getFullYear();
  return (
    <div className="empty">
      {art === 'leaves' ? (
        <div className="empty__art" aria-hidden="true" data-decor="true">
          <DateLeaf date={`${year}-03-14`} status="expired" />
          <DateLeaf date={`${year}-06-28`} status="expiring" />
          <DateLeaf date={`${year}-11-03`} status="valid" />
        </div>
      ) : art !== 'none' ? (
        <span className="empty__glyph" aria-hidden="true">
          <Icon name={GLYPHS[art]} size={30} />
        </span>
      ) : null}
      <h2 className="empty__title">{title}</h2>
      {description ? <p className="empty__text">{description}</p> : null}
      {action ? <div className="empty__actions">{action}</div> : null}
    </div>
  );
}

interface ErrorProps {
  error: unknown;
  title?: string | undefined;
  onRetry?: (() => void) | undefined;
  action?: ReactNode;
}

export function ErrorView({ error, title, onRetry, action }: ErrorProps) {
  return (
    <div className="empty" role="alert">
      <span className="empty__glyph empty__glyph--danger" aria-hidden="true">
        <Icon name={error instanceof NetworkError ? 'wifi' : 'alert'} size={30} />
      </span>
      <h2 className="empty__title">{title ?? 'Не удалось загрузить'}</h2>
      <p className="empty__text">{messageForError(error)}</p>
      {onRetry || action ? (
        <div className="empty__actions">
          {onRetry ? (
            <Button variant="secondary" icon="refresh" onClick={onRetry}>
              Повторить
            </Button>
          ) : null}
          {action}
        </div>
      ) : null}
    </div>
  );
}
