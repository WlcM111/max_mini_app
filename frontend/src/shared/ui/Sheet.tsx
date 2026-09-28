import { useEffect, useId, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

interface Props {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: ReactNode;
  children?: ReactNode;
}

/** Нижняя шторка (на широком экране — диалог по центру) с анимацией входа и выхода. */
export function Sheet({ open, onClose, title, description, children }: Props) {
  const titleId = useId();
  const [mounted, setMounted] = useState(open);
  const [closing, setClosing] = useState(false);

  useEffect(() => {
    if (open) {
      setMounted(true);
      setClosing(false);
      return undefined;
    }
    if (!mounted) return undefined;
    setClosing(true);
    const timer = window.setTimeout(() => {
      setMounted(false);
      setClosing(false);
    }, 220);
    return () => window.clearTimeout(timer);
  }, [open, mounted]);

  useEffect(() => {
    if (!open) return undefined;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = previous;
    };
  }, [open, onClose]);

  if (!mounted) return null;
  return createPortal(
    <div className="sheet-backdrop" data-state={closing ? 'closing' : 'open'} role="presentation" onClick={onClose}>
      <div className="sheet" role="dialog" aria-modal="true" aria-labelledby={titleId} onClick={(event) => event.stopPropagation()}>
        <span className="sheet__handle" aria-hidden="true" />
        <h2 className="sheet__title" id={titleId}>
          {title}
        </h2>
        {description ? <p className="sheet__text">{description}</p> : null}
        {children}
      </div>
    </div>,
    document.body,
  );
}
