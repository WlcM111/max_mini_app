import { useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { StatusBadge } from '../../shared/ui/StatusBadge';
import { formatDate, formatDateTime } from '../../shared/lib/dates';
import { openExternal } from '../../platform/max/links';
import { roleAllows, useRole } from '../../session/useSession';
import { getOrganization } from '../organizations/api';
import { deleteDocument, getDocument } from './api';

/** Карточка документа: срок, реквизиты, напоминание, история периодов, действия. */
export function DocumentCardPage() {
  const { docId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const document = useQuery({
    queryKey: queryKeys.document(docId),
    queryFn: () => getDocument(docId),
    enabled: docId !== '',
  });

  const organizationId = document.data?.organization_id ?? '';
  const role = useRole(organizationId);
  // Время напоминания показывается в часовом поясе организации (FR-13).
  const organization = useQuery({
    queryKey: queryKeys.organization(organizationId),
    queryFn: () => getOrganization(organizationId),
    enabled: organizationId !== '',
  });
  const timezone = organization.data?.timezone ?? 'Europe/Moscow';

  const remove = useMutation({
    mutationFn: () => deleteDocument(docId),
    onSuccess: async () => {
      queryClient.removeQueries({ queryKey: queryKeys.document(docId) });
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization(organizationId) });
      navigate(`/o/${organizationId}`, { replace: true });
    },
    onError: (error) => setNotice(messageForError(error)),
  });

  if (document.isLoading) {
    return (
      <AppShell title="Документ">
        <LoadingView rows={4} />
      </AppShell>
    );
  }
  if (document.isError || !document.data) {
    const notFound = document.error instanceof ApiError && document.error.status === 404;
    return (
      <AppShell title="Документ">
        <ErrorView
          error={document.error}
          title={notFound ? 'Документ удалён или недоступен' : undefined}
          onRetry={notFound ? undefined : () => void document.refetch()}
        />
        <Button size="medium" variant="secondary" onClick={() => navigate(-1)}>
          К списку
        </Button>
      </AppShell>
    );
  }

  const doc = document.data;
  const reminderText =
    doc.reminders_state === 'unavailable'
      ? 'План напоминаний временно недоступен'
      : doc.reminders_state === 'pending'
        ? 'Напоминания пересчитываются'
        : doc.next_reminder_at
          ? `Ближайшее напоминание: ${formatDateTime(doc.next_reminder_at, timezone)}`
          : 'Напоминания не запланированы';

  return (
    <AppShell
      title={doc.title}
      subtitle={doc.current_period.valid_until ? `Срок до ${formatDate(doc.current_period.valid_until)}` : 'Бессрочный'}
      actions={
        roleAllows(role, 'editor') ? (
          <>
            <Button size="large" stretched onClick={() => navigate(`/d/${doc.id}/renew`)}>
              Продлить
            </Button>
            <div className="row">
              <Button size="medium" stretched variant="secondary" onClick={() => navigate(`/d/${doc.id}/edit`)}>
                Изменить
              </Button>
              <Button size="medium" stretched variant="destructive" onClick={() => setConfirmOpen(true)}>
                Удалить
              </Button>
            </div>
          </>
        ) : null
      }
    >
      <div className="card stack stack--tight">
        <StatusBadge status={doc.status} daysLeft={doc.days_left} />
        <p className="card__text">{reminderText}</p>
        {doc.reminder_offsets_days.length > 0 ? (
          <p className="muted">
            Напоминания: {doc.reminder_offsets_days.map((d) => (d === 0 ? 'в день срока' : `за ${d} дн.`)).join(', ')}
          </p>
        ) : null}
      </div>

      <section className="card stack stack--tight" aria-label="Реквизиты">
        <h2 className="card__title">Реквизиты</h2>
        <Row label="Номер" value={doc.number} />
        <Row label="Кем выдан" value={doc.issuer} />
        <Row label="Ответственный" value={doc.responsible_label} />
        <Row label="Начало" value={doc.current_period.valid_from ? formatDate(doc.current_period.valid_from) : null} />
        <Row label="Заметки" value={doc.notes} />
        {doc.reference_url ? (
          <Button
            size="medium"
            variant="secondary"
            onClick={() => {
              const url = doc.reference_url;
              if (url) void openExternal(url).catch((error) => setNotice(messageForError(error)));
            }}
          >
            Открыть источник
          </Button>
        ) : null}
      </section>

      {doc.renewal_steps.length > 0 ? (
        <section className="card stack stack--tight" aria-label="Как продлить">
          <h2 className="card__title">Как продлить</h2>
          <ol className="stack stack--tight" style={{ paddingLeft: 18, margin: 0 }}>
            {doc.renewal_steps.map((step, index) => (
              <li key={index}>{step}</li>
            ))}
          </ol>
          {doc.data_status === 'model' ? <ModelDataBadge compact /> : null}
        </section>
      ) : null}

      {doc.periods.length > 1 ? (
        <section className="card stack stack--tight" aria-label="История периодов">
          <h2 className="card__title">История периодов</h2>
          {doc.periods.map((period) => (
            <p key={period.id} className="muted">
              {period.valid_from ? formatDate(period.valid_from) : '—'} —{' '}
              {period.valid_until ? formatDate(period.valid_until) : 'бессрочно'}
              {period.is_current ? ' · текущий' : ''}
            </p>
          ))}
        </section>
      ) : null}

      {notice ? (
        <p className="field__error" role="alert" aria-live="polite">
          {notice}
        </p>
      ) : null}

      <ConfirmDialog
        open={confirmOpen}
        title="Удалить документ?"
        description="Документ и его напоминания будут удалены без возможности восстановления."
        confirmLabel="Удалить"
        destructive
        pending={remove.isPending}
        onConfirm={() => remove.mutate()}
        onCancel={() => setConfirmOpen(false)}
      />
    </AppShell>
  );
}

function Row({ label, value }: { label: string; value: string | null | undefined }) {
  if (!value) return null;
  return (
    <p className="card__text">
      <span className="muted">{label}: </span>
      {value}
    </p>
  );
}
