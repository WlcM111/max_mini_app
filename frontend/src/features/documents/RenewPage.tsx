import { useEffect, useRef, useState } from 'react';
import { Button, Switch } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { DateField } from '../../shared/ui/DateField';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { shiftDate, todayInTimeZone } from '../../shared/lib/dates';
import { validateDates } from '../../shared/lib/validation';
import { uuidV4 } from '../../shared/lib/uuid';
import { haptic } from '../../platform/max/haptics';
import { setClosingConfirmation } from '../../platform/max/closingConfirmation';
import { getOrganization } from '../organizations/api';
import { getDocument, renewDocument } from './api';

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

  const document = useQuery({
    queryKey: queryKeys.document(docId),
    queryFn: () => getDocument(docId),
    enabled: docId !== '',
  });
  const organizationId = document.data?.organization_id ?? '';
  const organization = useQuery({
    queryKey: queryKeys.organization(organizationId),
    queryFn: () => getOrganization(organizationId),
    enabled: organizationId !== '',
  });

  useEffect(() => {
    if (!document.data || validFrom !== null) return;
    const timezone = organization.data?.timezone ?? 'Europe/Moscow';
    const today = todayInTimeZone(timezone);
    const previousUntil = document.data.current_period.valid_until;
    const start = previousUntil && previousUntil >= today ? shiftDate(previousUntil, { days: 1 }) : today;
    setValidFrom(start);
    setValidUntil(shiftDate(start, { years: 1 }));
  }, [document.data, organization.data, validFrom]);

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
      navigate(`/d/${docId}`, { replace: true });
    },
    onError: (failure) => {
      void haptic('error');
      if (failure instanceof ApiError && failure.code === 'CONFLICT_ID_REUSED') periodId.current = uuidV4();
      setError(messageForError(failure));
    },
  });

  if (document.isLoading) {
    return (
      <AppShell title="Продление">
        <LoadingView rows={3} />
      </AppShell>
    );
  }
  if (document.isError || !document.data) {
    return (
      <AppShell title="Продление">
        <ErrorView error={document.error} onRetry={() => void document.refetch()} />
      </AppShell>
    );
  }

  const submit = () => {
    if (submitting.current || saved.current || renew.isPending) return;
    const dates = validateDates(validFrom, indefinite ? null : validUntil);
    if (dates) {
      setError(dates);
      return;
    }
    setError(null);
    submitting.current = true;
    renew.mutate(undefined, { onSettled: () => { submitting.current = false; } });
  };

  return (
    <AppShell
      title="Продление"
      subtitle={document.data.title}
      actions={
        <Button size="large" stretched loading={renew.isPending} disabled={renew.isPending} onClick={submit}>
          Сохранить новый срок
        </Button>
      }
    >
      <p className="card__text">
        Прежний период останется в истории документа, напоминания будут пересчитаны по новому сроку.
      </p>
      <DateField label="Новый период с" value={validFrom} onChange={setValidFrom} />
      <div className="field">
        <div className="row row--between">
          <span className="field__label">Бессрочный документ</span>
          <Switch
            checked={indefinite}
            aria-label="Бессрочный документ"
            onChange={(event) => setIndefinite(event.target.checked)}
          />
        </div>
      </div>
      {!indefinite ? (
        <DateField
          label="Новый срок до"
          value={validUntil}
          min={validFrom ?? undefined}
          onChange={setValidUntil}
          error={error ?? undefined}
        />
      ) : null}
      {error && indefinite ? (
        <p className="field__error" role="alert" aria-live="polite">
          {error}
        </p>
      ) : null}
    </AppShell>
  );
}
