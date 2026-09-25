import { useMemo, useState } from 'react';
import { Button, Switch } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import type { DocumentCreate } from '../../api/client';
import { AppShell } from '../../shared/ui/AppShell';
import { DateField } from '../../shared/ui/DateField';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { uuidV4 } from '../../shared/lib/uuid';
import { validateDates } from '../../shared/lib/validation';
import { getCatalog } from '../organizations/api';
import { createDocumentsBatch } from '../documents/api';
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
  const queryClient = useQueryClient();
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

  if (catalog.isLoading) {
    return (
      <AppShell title="Сроки документов">
        <LoadingView rows={3} />
      </AppShell>
    );
  }
  if (catalog.isError) {
    return (
      <AppShell title="Сроки документов">
        <ErrorView error={catalog.error} onRetry={() => void catalog.refetch()} />
      </AppShell>
    );
  }

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
      subtitle="Шаг 4 из 4"
      actions={
        <Button size="large" stretched loading={save.isPending} onClick={submit}>
          Сохранить документы
        </Button>
      }
    >
      <p className="card__text">Дату можно не указывать — вернётесь к ней позже в карточке документа.</p>
      {rows.map((entry, index) => (
        <section key={entry.id} className="card stack stack--tight" aria-label={entry.title}>
          <h2 className="card__title">{entry.title}</h2>
          <div className="row row--between">
            <span>Бессрочный</span>
            <Switch
              checked={entry.indefinite}
              aria-label={`${entry.title}: бессрочный`}
              onChange={(event) => {
                const checked = event.target.checked;
                setEntries(
                  rows.map((item, position) =>
                    position === index ? { ...item, indefinite: checked, validUntil: checked ? null : item.validUntil } : item,
                  ),
                );
              }}
            />
          </div>
          {!entry.indefinite ? (
            <DateField
              label="Действует до"
              value={entry.validUntil}
              onChange={(value) =>
                setEntries(rows.map((item, position) => (position === index ? { ...item, validUntil: value } : item)))
              }
            />
          ) : null}
        </section>
      ))}
      {error ? (
        <p className="field__error" role="alert" aria-live="polite">
          {error}
        </p>
      ) : null}
    </AppShell>
  );
}
