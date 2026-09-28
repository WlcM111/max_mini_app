import { useEffect, useRef } from 'react';
import { Button } from './Button';
import { Sheet } from './Sheet';

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

/** Подтверждение необратимого действия. */
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
    if (!open) return undefined;
    const timer = window.setTimeout(() => confirmRef.current?.focus(), 80);
    return () => window.clearTimeout(timer);
  }, [open]);
  return (
    <Sheet open={open} onClose={pending ? () => undefined : onCancel} title={title} description={description}>
      <div className="sheet__actions">
        <Button ref={confirmRef} size="l" stretched variant={destructive ? 'danger' : 'primary'} loading={pending} onClick={onConfirm}>
          {confirmLabel}
        </Button>
        <Button size="l" stretched variant="neutral" disabled={pending} onClick={onCancel}>
          {cancelLabel}
        </Button>
      </div>
    </Sheet>
  );
}
