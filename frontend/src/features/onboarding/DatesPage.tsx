import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useLocation, useNavigate } from 'react-router';
import type { DocumentCreate } from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { DateField } from '../../shared/ui/DateField';
import { Icon } from '../../shared/ui/Icon';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { Steps } from '../../shared/ui/Steps';
import { ToggleRow } from '../../shared/ui/Switch';
import { toast } from '../../shared/ui/Toast';
import { validateDates } from '../../shared/lib/validation';
import { uuidV4 } from '../../shared/lib/uuid';
import { createDocumentsBatch } from '../documents/api';
import { getCatalog } from '../organizations/api';
import { getDraft, resetDraft } from './onboardingDraft';

interface Entry {
  id: string;
  typeCode: string;
  title: string;
  validUntil: string | null;
  indefinite: boolean;
  offsets: number[];
}

/** Шаг 4: сроки выбранных документов; пакет создаётся одним запросом (FR-08). */
export function DatesPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const fromDashboard = (location.state as { from?: string } | null)?.from === 'dashboard';
  const draft = getDraft();
  const [error, setError] = useState<string | null>(null);
  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });

  const initial = useMemo<Entry[]>(() => {
    const types = catalog.data?.document_types ?? [];
    return draft.selectedTypes.map((code) => {
      const type = types.find((item) => item.code === code);
      return {
        id: uuidV4(),
        typeCode: code,
        title: type?.title ?? code,
        validUntil: null,
        indefinite: false,
        offsets: type && type.default_reminder_offsets_days.length > 0 ? [...type.default_reminder_offsets_days] : [30, 7, 1],
      };
    });
  }, [catalog.data, draft.selectedTypes]);
  const [entries, setEntries] = useState<Entry[] | null>(null);
  const rows = entries ?? initial;

  const save = useMutation({
    mutationFn: () => {
      const items: DocumentCreate[] = rows.map((entry) => ({
        id: entry.id,
        document_type_code: entry.typeCode,
        title: entry.title,
        number: null,
        issuer: null,
        responsible_label: null,
        notes: null,
        reference_url: null,
        valid_from: null,
        valid_until: entry.indefinite ? null : entry.validUntil,
        reminder_offsets_days: entry.offsets,
      }));
      return createDocumentsBatch(draft.organizationId, items);
    },
    onSuccess: async () => {
      const organizationId = draft.organizationId;
      resetDraft();
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization(organizationId) });
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.suggestions(organizationId) });
      toast('Документы добавлены', 'success');
      navigate(`/o/${organizationId}`, { replace: true });
    },
    onError: (failure) => {
      if (failure instanceof ApiError && failure.code === 'CONFLICT_ID_REUSED') {
        setEntries(rows.map((entry) => ({ ...entry, id: uuidV4() })));
        setError('Повторите сохранение: идентификаторы обновлены.');
        return;
      }
      setError(messageForError(failure));
    },
  });

  const steps = fromDashboard ? null : <Steps current={4} total={4} />;

  if (catalog.isLoading) {
    return (
      <AppShell title="Сроки документов" headerExtra={steps}>
        <LoadingView rows={3} variant="form" />
      </AppShell>
    );
  }

  if (catalog.isError) {
    return (
      <AppShell title="Сроки документов" headerExtra={steps}>
        <ErrorView error={catalog.error} onRetry={() => void catalog.refetch()} />
      </AppShell>
    );
  }

  if (rows.length === 0) {
    return (
      <AppShell title="Сроки документов" headerExtra={steps}>
        <EmptyView
          art="none"
          title="Документы не выбраны"
          action={
            <Button variant="secondary" onClick={() => navigate('/', { replace: true })}>
              На главную
            </Button>
          }
        />
      </AppShell>
    );
  }

  const updateEntry = (index: number, patch: Partial<Entry>) =>
    setEntries(rows.map((entry, position) => (position === index ? { ...entry, ...patch } : entry)));

  const submit = () => {
    for (const entry of rows) {
      const invalid = validateDates(null, entry.indefinite ? null : entry.validUntil);
      if (invalid) {
        setError(`${entry.title}: ${invalid}`);
        return;
      }
    }
    setError(null);
    save.mutate();
  };

  return (
    <AppShell
      title="Сроки документов"
      subtitle="Дату можно не указывать — вернётесь к ней позже в карточке документа."
      headerExtra={steps}
      actionsNote={
        error ? (
          <p className="actionbar__note" role="alert">
            <Icon name="alert" size={18} />
            {error}
          </p>
        ) : null
      }
      actions={
        <Button size="l" stretched loading={save.isPending} onClick={submit}>
          Сохранить документы ({rows.length})
        </Button>
      }
    >
      <div className="list stagger">
        {rows.map((entry, index) => (
          <section key={entry.id} className="form-card" aria-label={entry.title}>
            <h2 className="form-card__title">{entry.title}</h2>
            {!entry.indefinite ? (
              <DateField label="Действует до" value={entry.validUntil} onChange={(value) => updateEntry(index, { validUntil: value })} />
            ) : null}
            <ToggleRow
              flush
              title="Бессрочный документ"
              checked={entry.indefinite}
              onChange={(checked) => updateEntry(index, { indefinite: checked, validUntil: checked ? null : entry.validUntil })}
            />
          </section>
        ))}
      </div>
    </AppShell>
  );
}
