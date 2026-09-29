import { useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
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
import { plural } from '../../shared/lib/plural';
import { validateDates } from '../../shared/lib/validation';
import { uuidV4 } from '../../shared/lib/uuid';
import { createDocumentsBatch } from '../documents/api';
import { getCatalog } from '../organizations/api';
import { clearPick, readPick, type PickScope } from './documentPick';
import { getDraft, resetDraft } from './onboardingDraft';

const BATCH = 30; // domain.MaxDocumentsPerBatch
const DEFAULT_OFFSETS = [30, 7, 1];
const DOCS: [string, string, string] = ['документ', 'документа', 'документов'];

interface Entry {
  id: string;
  typeCode: string | null; // null — свой документ без типа
  title: string;
  validUntil: string | null;
  indefinite: boolean;
  offsets: number[];
}

/** Шаг 4: сроки выбранных документов; сохраняются пакетами до 30 штук (FR-05, FR-27). */
export function DatesPage() {
  const navigate = useNavigate();
  const { orgId } = useParams();
  const queryClient = useQueryClient();
  const inOrganization = Boolean(orgId);
  const scope = useMemo<PickScope>(() => (orgId ? { kind: 'organization', organizationId: orgId } : { kind: 'onboarding' }), [orgId]);
  const organizationId = orgId ?? getDraft().organizationId;
  const [pick] = useState(() => readPick(scope));
  const [error, setError] = useState<string | null>(null);
  const saved = useRef(new Set<string>());
  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });

  const initial = useMemo<Entry[]>(() => {
    const types = catalog.data?.document_types ?? [];
    const typical = pick.selectedTypes.map((code) => {
      const type = types.find((item) => item.code === code);
      return {
        id: uuidV4(),
        typeCode: code,
        title: type?.title ?? code,
        validUntil: null,
        indefinite: false,
        offsets: type && type.default_reminder_offsets_days.length > 0 ? [...type.default_reminder_offsets_days] : DEFAULT_OFFSETS,
      };
    });
    const custom = pick.customDocuments.map((item) => ({
      id: item.id,
      typeCode: null,
      title: item.title,
      validUntil: null,
      indefinite: false,
      offsets: DEFAULT_OFFSETS,
    }));
    return [...typical, ...custom];
  }, [catalog.data, pick]);
  const [entries, setEntries] = useState<Entry[] | null>(null);
  const rows = entries ?? initial;

  const save = useMutation({
    // Пакеты по 30: уже сохранённые при повторе не отправляются, id остаются прежними.
    mutationFn: async () => {
      const pending = rows.filter((entry) => !saved.current.has(entry.id));
      for (let start = 0; start < pending.length; start += BATCH) {
        const chunk = pending.slice(start, start + BATCH);
        const items: DocumentCreate[] = chunk.map((entry) => ({
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
        await createDocumentsBatch(organizationId, items);
        chunk.forEach((entry) => saved.current.add(entry.id));
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization(organizationId) });
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.suggestions(organizationId) });
      if (inOrganization) {
        clearPick(scope);
        toast(`Добавлено: ${rows.length} ${plural(rows.length, DOCS)}`, 'success');
        navigate(-2); // назад к экрану, откуда начали подбор
        return;
      }
      resetDraft();
      toast('Документы добавлены', 'success');
      navigate(`/o/${organizationId}`, { replace: true });
    },
    onError: (failure) => {
      if (failure instanceof ApiError && failure.code === 'CONFLICT_ID_REUSED') {
        setEntries(rows.map((entry) => (saved.current.has(entry.id) ? entry : { ...entry, id: uuidV4() })));
        setError('Повторите сохранение: идентификаторы обновлены.');
        return;
      }
      setError(messageForError(failure));
    },
  });

  const steps = inOrganization ? null : <Steps current={4} total={4} />;

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
            <Button variant="secondary" onClick={() => navigate(inOrganization ? `/o/${organizationId}/documents` : '/', { replace: true })}>
              {inOrganization ? 'К документам' : 'На главную'}
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
      layout={inOrganization ? 'stack' : 'split'}
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
            {entry.typeCode === null ? <span className="pill pill--accent">Свой документ</span> : null}
            {!entry.indefinite ? (
              <DateField
                label="Действует до"
                value={entry.validUntil}
                pickerDescription={entry.title}
                onChange={(value) => updateEntry(index, { validUntil: value })}
              />
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
