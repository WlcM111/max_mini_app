import { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Banner } from '../../shared/ui/Banner';
import { Button } from '../../shared/ui/Button';
import { DateField } from '../../shared/ui/DateField';
import { DateLeaf } from '../../shared/ui/DateLeaf';
import { Icon } from '../../shared/ui/Icon';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { StatusBadge } from '../../shared/ui/StatusBadge';
import { ToggleRow } from '../../shared/ui/Switch';
import { toast } from '../../shared/ui/Toast';
import { formatDate, shiftDate, todayInTimeZone } from '../../shared/lib/dates';
import { plural } from '../../shared/lib/plural';
import { validateDates } from '../../shared/lib/validation';
import { uuidV4 } from '../../shared/lib/uuid';
import { haptic } from '../../platform/max/haptics';
import { setClosingConfirmation } from '../../platform/max/closingConfirmation';
import { getOrganization } from '../organizations/api';
import { getDocument, renewDocument } from './api';

const PRESET_YEARS = [1, 2, 3, 5];

/** Продление документа: новый текущий период (FR-09). */
export function RenewPage() {
  const { docId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const periodId = useRef(uuidV4());
  const submitting = useRef(false);
  const saved = useRef(false);
  const [validFrom, setValidFrom] = useState<string | null>(null);
  const [validUntil, setValidUntil] = useState<string | null>(null);
  const [indefinite, setIndefinite] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const documentQuery = useQuery({
    queryKey: queryKeys.document(docId),
    queryFn: () => getDocument(docId),
    enabled: docId !== '',
  });
  const organizationId = documentQuery.data?.organization_id ?? '';
  const organization = useQuery({
    queryKey: queryKeys.organization(organizationId),
    queryFn: () => getOrganization(organizationId),
    enabled: organizationId !== '',
  });

  useEffect(() => {
    if (!documentQuery.data || validFrom !== null) return;
    const timezone = organization.data?.timezone ?? 'Europe/Moscow';
    const today = todayInTimeZone(timezone);
    const previousUntil = documentQuery.data.current_period.valid_until;
    const start = previousUntil && previousUntil >= today ? shiftDate(previousUntil, { days: 1 }) : today;
    setValidFrom(start);
    setValidUntil(shiftDate(start, { years: 1 }));
  }, [documentQuery.data, organization.data, validFrom]);

  useEffect(() => {
    void setClosingConfirmation(true);
    return () => {
      void setClosingConfirmation(false);
    };
  }, []);

  const renew = useMutation({
    mutationFn: () =>
      renewDocument(docId, {
        id: periodId.current,
        valid_from: validFrom,
        valid_until: indefinite ? null : validUntil,
      }),
    onSuccess: async (updated) => {
      saved.current = true;
      void haptic('success');
      await setClosingConfirmation(false);
      queryClient.setQueryData(queryKeys.document(docId), updated);
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization(organizationId) });
      toast('Срок продлён', 'success');
      navigate(`/d/${docId}`, { replace: true });
    },
    onError: (failure) => {
      void haptic('error');
      if (failure instanceof ApiError && failure.code === 'CONFLICT_ID_REUSED') periodId.current = uuidV4();
      setError(messageForError(failure));
    },
  });

  if (documentQuery.isLoading) {
    return (
      <AppShell title="Продление">
        <LoadingView rows={3} variant="form" />
      </AppShell>
    );
  }

  if (documentQuery.isError || !documentQuery.data) {
    return (
      <AppShell title="Продление">
        <ErrorView error={documentQuery.error} onRetry={() => void documentQuery.refetch()} />
      </AppShell>
    );
  }

  const doc = documentQuery.data;
  const currentUntil = doc.current_period.valid_until ?? null;

  const submit = () => {
    if (submitting.current || saved.current || renew.isPending) return;
    const dates = validateDates(validFrom, indefinite ? null : validUntil);
    if (dates) {
      setError(dates);
      void haptic('error');
      return;
    }
    setError(null);
    submitting.current = true;
    renew.mutate(undefined, {
      onSettled: () => {
        submitting.current = false;
      },
    });
  };

  return (
    <AppShell
      title="Продление"
      subtitle={doc.title}
      actionsNote={
        error ? (
          <p className="actionbar__note" role="alert" aria-live="polite">
            <Icon name="alert" size={18} />
            {error}
          </p>
        ) : null
      }
      actions={
        <Button size="l" stretched icon="check" loading={renew.isPending} onClick={submit}>
          Сохранить новый срок
        </Button>
      }
    >
      <section className="group" aria-labelledby="renew-current-title">
        <h2 className="group__title" id="renew-current-title">
          Сейчас
        </h2>
        <div className="group__card group__card--pad">
          <div className="current-period">
            <DateLeaf date={currentUntil} status={doc.status} />
            <div className="current-period__text">
              <span className="current-period__date">{currentUntil ? `Действует до ${formatDate(currentUntil)}` : 'Бессрочный документ'}</span>
              <StatusBadge status={doc.status} daysLeft={doc.days_left} />
            </div>
          </div>
        </div>
      </section>

      <section className="group" aria-labelledby="renew-next-title">
        <h2 className="group__title" id="renew-next-title">
          Новый период
        </h2>
        <div className="form-card">
          <DateField label="Новый период с" value={validFrom} onChange={setValidFrom} />
          <ToggleRow flush title="Бессрочный документ" checked={indefinite} onChange={setIndefinite} />
          {!indefinite ? (
            <div className="reveal field">
              <DateField label="Новый срок до" value={validUntil} min={validFrom ?? undefined} onChange={setValidUntil} />
              <div className="chips" role="group" aria-label="Быстрый выбор срока">
                {PRESET_YEARS.map((years) => {
                  const target = validFrom ? shiftDate(validFrom, { years }) : null;
                  return (
                    <button
                      key={years}
                      type="button"
                      className="chip"
                      aria-pressed={target !== null && target === validUntil}
                      disabled={!target}
                      onClick={() => setValidUntil(target)}
                    >
                      +{years} {plural(years, ['год', 'года', 'лет'])}
                    </button>
                  );
                })}
              </div>
            </div>
          ) : null}
        </div>
      </section>

      <Banner tone="info" icon="history">
        Прежний период останется в истории документа, а напоминания пересчитаются по новому сроку.
      </Banner>
    </AppShell>
  );
}
