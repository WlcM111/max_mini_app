import { useEffect, useRef } from 'react';
import { Button } from '@maxhub/max-ui';

interface Props {
  open: boolean;
  title: string;
  description?: string;
  confirmLabel: string;
  cancelLabel?: string;
  destructive?: boolean;
  pending?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/** Подтверждение необратимых действий: удаление, выход, отзыв приглашения. */
export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Отмена',
  destructive = false,
  pending = false,
  onConfirm,
  onCancel,
}: Props) {
  const confirmRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (open) confirmRef.current?.focus();
  }, [open]);

  if (!open) return null;
  return (
    <div className="dialog-backdrop" role="presentation" onClick={onCancel}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(event) => event.stopPropagation()}
      >
        <h2 className="card__title">{title}</h2>
        {description ? <p className="card__text">{description}</p> : null}
        <Button
          ref={confirmRef}
          size="large"
          stretched
          loading={pending}
          variant={destructive ? 'destructive' : 'primary'}
          onClick={onConfirm}
        >
          {confirmLabel}
        </Button>
        <Button size="large" stretched variant="secondary" onClick={onCancel} disabled={pending}>
          {cancelLabel}
        </Button>
      </div>
    </div>
  );
}
