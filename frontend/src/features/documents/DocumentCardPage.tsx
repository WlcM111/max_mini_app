import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import type { Document } from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { DateLeaf } from '../../shared/ui/DateLeaf';
import { Icon } from '../../shared/ui/Icon';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { KeyValue } from '../../shared/ui/Rows';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { StatusBadge } from '../../shared/ui/StatusBadge';
import { toast } from '../../shared/ui/Toast';
import { cx } from '../../shared/lib/cx';
import { formatDate, formatDateTime, todayInTimeZone } from '../../shared/lib/dates';
import { DAY_FORMS, heroCount, periodProgress } from '../../shared/lib/format';
import { pluralWithCount } from '../../shared/lib/plural';
import { openExternal } from '../../platform/max/links';
import { roleAllows, useRole } from '../../session/useSession';
import { getCatalog, getOrganization } from '../organizations/api';
import { deleteDocument, getDocument } from './api';
import { useMemberNames } from '../members/useMemberNames';
import { clearChecklist, progressText, readChecklist, saveChecklist, toggleStep } from './renewalChecklist';

/** Главный блок карточки: крупный листок, обратный отсчёт, статус и шкала периода. */
function DocHero({ doc, timezone }: { doc: Document; timezone: string }) {
  const until = doc.current_period.valid_until ?? null;
  const from = doc.current_period.valid_from ?? null;
  const count = heroCount(doc.status === 'no_expiry' ? null : doc.days_left);
  const progress = from && until ? periodProgress(from, until, todayInTimeZone(timezone)) : null;
  return (
    <section className={`doc-hero doc-hero--${doc.status}`} aria-label="Срок действия">
      <div className="doc-hero__top">
        <DateLeaf date={until} status={doc.status} size="l" />
        <div className="doc-hero__count">
          {count.value ? <span className={cx('doc-hero__num', count.word && 'doc-hero__num--word')}>{count.value}</span> : null}
          <span className="doc-hero__unit">{count.unit}</span>
        </div>
      </div>
      <div className="doc-hero__row">
        <StatusBadge status={doc.status} daysLeft={doc.days_left} withDays={false} />
        {progress === null ? <span className="doc-hero__date">{until ? `до ${formatDate(until)}` : 'без даты окончания'}</span> : null}
      </div>
      {progress !== null && from && until ? (
        <div className="period" aria-label={`Период действия с ${formatDate(from)} по ${formatDate(until)}`}>
          <div className="period__track">
            <span className="period__fill" style={{ width: `${progress}%` }} />
          </div>
          <div className="period__labels" aria-hidden="true">
            <span>с {formatDate(from)}</span>
            <span>до {formatDate(until)}</span>
          </div>
        </div>
      ) : null}
    </section>
  );
}

/** Карточка документа: срок, напоминания, реквизиты, чек-лист продления, история периодов. */
export function DocumentCardPage() {
  const { docId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [checklist, setChecklist] = useState<number[]>([]);

  const documentQuery = useQuery({
    queryKey: queryKeys.document(docId),
    queryFn: () => getDocument(docId),
    enabled: docId !== '',
  });
  const organizationId = documentQuery.data?.organization_id ?? '';
  const role = useRole(organizationId);
  const canEdit = roleAllows(role, 'editor');
  // Время напоминания показывается в часовом поясе организации (FR-13).
  const organization = useQuery({
    queryKey: queryKeys.organization(organizationId),
    queryFn: () => getOrganization(organizationId),
    enabled: organizationId !== '',
  });
  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });
  const timezone = organization.data?.timezone ?? 'Europe/Moscow';
  const names = useMemberNames(organizationId, Boolean(documentQuery.data?.responsible_account_id));
  const responsibleName = names.get(documentQuery.data?.responsible_account_id ?? '');

  // Чек-лист продления: отметки хранятся на устройстве пользователя (FR-20).
  const documentId = documentQuery.data?.id ?? '';
  const stepCount = documentQuery.data?.renewal_steps.length ?? 0;
  useEffect(() => {
    if (documentId === '') return;
    setChecklist(readChecklist(documentId, stepCount));
  }, [documentId, stepCount]);

  const toggleChecklistStep = (index: number) => {
    const next = toggleStep(checklist, index);
    setChecklist(next);
    saveChecklist(documentId, next);
  };
  const resetChecklist = () => {
    setChecklist([]);
    clearChecklist(documentId);
  };

  const remove = useMutation({
    mutationFn: () => deleteDocument(docId),
    onSuccess: async () => {
      queryClient.removeQueries({ queryKey: queryKeys.document(docId) });
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization(organizationId) });
      toast('Документ удалён', 'success');
      navigate(`/o/${organizationId}`, { replace: true });
    },
    onError: (error) => {
      setConfirmOpen(false);
      toast(messageForError(error), 'error');
    },
  });

  if (documentQuery.isLoading) {
    return (
      <AppShell title="Документ" titleSkeleton>
        <LoadingView rows={2} variant="card" />
      </AppShell>
    );
  }

  if (documentQuery.isError || !documentQuery.data) {
    const notFound = documentQuery.error instanceof ApiError && documentQuery.error.status === 404;
    return (
      <AppShell title="Документ">
        <ErrorView
          error={documentQuery.error}
          title={notFound ? 'Документ удалён или недоступен' : undefined}
          onRetry={notFound ? undefined : () => void documentQuery.refetch()}
          action={
            <Button variant="neutral" onClick={() => navigate(-1)}>
              К списку
            </Button>
          }
        />
      </AppShell>
    );
  }

  const doc = documentQuery.data;
  const typeTitle = catalog.data?.document_types.find((type) => type.code === doc.document_type_code)?.title;
  const reminderText =
    doc.reminders_state === 'unavailable'
      ? 'План напоминаний временно недоступен'
      : doc.reminders_state === 'pending'
        ? 'Напоминания пересчитываются'
        : doc.next_reminder_at
          ? `Ближайшее напоминание: ${formatDateTime(doc.next_reminder_at, timezone)}`
          : 'Напоминания не запланированы';
  const stepsDone = checklist.length;
  const allDone = stepCount > 0 && stepsDone >= stepCount;
  let sourceHost = '';
  try {
    sourceHost = doc.reference_url ? new URL(doc.reference_url).host : '';
  } catch {
    sourceHost = doc.reference_url ?? '';
  }
  const hasDetails = Boolean(
    doc.number || doc.issuer || doc.responsible_label || responsibleName || doc.current_period.valid_from || doc.notes || doc.reference_url,
  );

  const actions = canEdit ? (
    <>
      <Button size="l" stretched icon="refresh" onClick={() => navigate(`/d/${doc.id}/renew`)}>
        Продлить
      </Button>
      <div className="actionbar__row">
        <Button variant="neutral" icon="edit" onClick={() => navigate(`/d/${doc.id}/edit`)}>
          Изменить
        </Button>
        <Button variant="danger-soft" icon="trash" onClick={() => setConfirmOpen(true)}>
          Удалить
        </Button>
      </div>
    </>
  ) : undefined;

  return (
    <AppShell title={doc.title} subtitle={typeTitle && typeTitle !== doc.title ? typeTitle : undefined} actions={actions} layout="columns">
      <DocHero doc={doc} timezone={timezone} />

      <section className="group" aria-labelledby="reminders-title">
        <h2 className="group__title" id="reminders-title">
          Напоминания
        </h2>
        <div className="group__card">
          <div className={cx('info-row', doc.reminders_state !== 'actual' && 'info-row--warn')}>
            <span className="info-row__icon">
              <Icon name={doc.reminders_state === 'actual' ? 'bell' : 'refresh'} />
            </span>
            <div className="info-row__body">
              <p className="info-row__text">{reminderText}</p>
              {responsibleName ? <p className="info-row__hint">Получает лично: {responsibleName}</p> : null}
              {doc.reminder_offsets_days.length > 0 ? (
                <div className="pills" aria-label="Когда напомнить">
                  {doc.reminder_offsets_days.map((days) => (
                    <span key={days} className="pill">
                      {days === 0 ? 'в день срока' : `за ${pluralWithCount(days, DAY_FORMS)}`}
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </section>

      {hasDetails ? (
        <section className="group" aria-labelledby="details-title">
          <h2 className="group__title" id="details-title">
            Реквизиты
          </h2>
          <div className="group__card">
            <KeyValue label="Номер" value={doc.number} />
            <KeyValue label="Кем выдан" value={doc.issuer} />
            <KeyValue label="Ответственный" value={responsibleName ?? doc.responsible_label} />
            {responsibleName && doc.responsible_label ? <KeyValue label="Должность" value={doc.responsible_label} /> : null}
            <KeyValue label="Действует с" value={doc.current_period.valid_from ? formatDate(doc.current_period.valid_from) : null} />
            <KeyValue label="Заметки" value={doc.notes} stacked />
            {doc.reference_url ? (
              <button type="button" className="nav-row" onClick={() => void openExternal(doc.reference_url ?? '')}>
                <span className="nav-row__icon">
                  <Icon name="external" />
                </span>
                <span className="nav-row__text">
                  <span className="nav-row__title">Открыть источник</span>
                  <span className="nav-row__hint">{sourceHost}</span>
                </span>
                <Icon name="chevron-right" className="nav-row__chev" />
              </button>
            ) : null}
          </div>
        </section>
      ) : null}

      {stepCount > 0 ? (
        <section className="group" aria-labelledby="renewal-title">
          <div className="group__head">
            <h2 className="group__title" id="renewal-title">
              Как продлить
            </h2>
            <span className={cx('progress-badge', allDone && 'progress-badge--done')}>{progressText(stepsDone, stepCount)}</span>
          </div>
          <div className="group__card group__card--pad">
            <div className="progress" aria-hidden="true">
              <span className="progress__fill" style={{ transform: `scaleX(${stepCount ? stepsDone / stepCount : 0})` }} />
            </div>
            <ul className="checklist" aria-label="Шаги продления">
              {doc.renewal_steps.map((step, index) => (
                <li key={index}>
                  <label className="check-item">
                    <input
                      type="checkbox"
                      className="check-item__input"
                      checked={checklist.includes(index)}
                      onChange={() => toggleChecklistStep(index)}
                    />
                    <span className="check-item__box" aria-hidden="true">
                      <span className="check-item__num">{index + 1}</span>
                      <svg className="check-item__tick" viewBox="0 0 24 24" focusable="false">
                        <path d="M5 12.5l4.5 4.5L19 7.5" />
                      </svg>
                    </span>
                    <span className="check-item__text">{step}</span>
                  </label>
                </li>
              ))}
            </ul>
            <div className="checklist-foot">
              <span className="field__hint">Отметки видны только вам</span>
              {stepsDone > 0 ? (
                <Button variant="tertiary" size="s" onClick={resetChecklist}>
                  Снять отметки
                </Button>
              ) : null}
            </div>
            {doc.data_status === 'model' ? <ModelDataBadge compact /> : null}
          </div>
        </section>
      ) : null}

      {doc.periods.length > 1 ? (
        <section className="group" aria-labelledby="history-title">
          <h2 className="group__title" id="history-title">
            История периодов
          </h2>
          <div className="group__card group__card--pad">
            <ol className="timeline">
              {doc.periods.map((period) => (
                <li key={period.id} className={cx('timeline__item', period.is_current && 'timeline__item--current')}>
                  <span className="timeline__dot" aria-hidden="true" />
                  <span className="timeline__text">
                    {period.valid_from ? formatDate(period.valid_from) : 'без даты начала'} —{' '}
                    {period.valid_until ? formatDate(period.valid_until) : 'бессрочно'}
                  </span>
                  {period.is_current ? <span className="pill pill--accent">текущий</span> : null}
                </li>
              ))}
            </ol>
          </div>
        </section>
      ) : null}

      <ConfirmDialog
        open={confirmOpen}
        title="Удалить документ?"
        description="Документ, история периодов и запланированные напоминания будут удалены без возможности восстановления."
        confirmLabel="Удалить"
        destructive
        pending={remove.isPending}
        onConfirm={() => remove.mutate()}
        onCancel={() => setConfirmOpen(false)}
      />
    </AppShell>
  );
}
