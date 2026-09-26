import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Input, Switch, Textarea } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { DateField } from '../../shared/ui/DateField';
import { OffsetChips } from '../../shared/ui/OffsetChips';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { uuidV4 } from '../../shared/lib/uuid';
import { setClosingConfirmation } from '../../platform/max/closingConfirmation';
import { scanCode } from '../../platform/max/codeReader';
import { getBridge } from '../../platform/max/bridge';
import { haptic } from '../../platform/max/haptics';
import { getCatalog } from '../organizations/api';
import { useSession } from '../../session/useSession';
import { createDocument, draftDocument, getDocument, updateDocument } from './api';
import {
  emptyDocumentForm,
  formFromDocument,
  isDirty,
  toCreateBody,
  toUpdateBody,
  validateDocumentForm,
  type DocumentFormState,
  type FormErrors,
} from './documentForm';

interface Props {
  mode: 'create' | 'edit';
}

/** Форма документа: создание и изменение с защитой от повторной отправки. */
export function DocumentFormPage({ mode }: Props) {
  const { orgId = '', docId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  // Идентификатор создаётся один раз при открытии формы: повтор отправки
  // не создаёт дубль (идемпотентность по клиентскому UUID).
  const newId = useRef(uuidV4());
  // Синхронный замок: защищает от двойного нажатия до того, как состояние
  // мутации станет pending (архитектура §8).
  const submitting = useRef(false);
  // После успешного сохранения форма больше не отправляется: повторное нажатие
  // (в том числе двойной клик) не создаёт второй документ.
  const saved = useRef(false);
  const [form, setForm] = useState<DocumentFormState>(() => emptyDocumentForm());
  const [initial, setInitial] = useState<DocumentFormState>(() => emptyDocumentForm());
  const [errors, setErrors] = useState<FormErrors>({});
  const [canScan, setCanScan] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [assistantText, setAssistantText] = useState('');
  const { me } = useSession();

  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });
  const existing = useQuery({
    queryKey: queryKeys.document(docId),
    queryFn: () => getDocument(docId),
    enabled: mode === 'edit' && docId !== '',
  });

  const organizationId = mode === 'edit' ? (existing.data?.organization_id ?? '') : orgId;

  useEffect(() => {
    void getBridge().then((bridge) => setCanScan(bridge.canScanQr));
  }, []);

  useEffect(() => {
    if (mode === 'edit' && existing.data) {
      const loaded = formFromDocument(existing.data);
      setForm(loaded);
      setInitial(loaded);
    }
  }, [mode, existing.data]);

  const dirty = useMemo(() => isDirty(initial, form), [initial, form]);

  useEffect(() => {
    void setClosingConfirmation(dirty);
    return () => {
      void setClosingConfirmation(false);
    };
  }, [dirty]);

  // Быстрый ввод: ассистент распознаёт реквизиты и заполняет форму (FR-21).
  // Ничего не сохраняет — пользователь проверяет поля и нажимает «Сохранить».
  const recognize = useMutation({
    mutationFn: () => draftDocument(organizationId, assistantText),
    onSuccess: (draft) => {
      setForm((prev) => ({
        ...prev,
        title: draft.title !== '' ? draft.title : prev.title,
        number: draft.number ?? prev.number,
        issuer: draft.issuer ?? prev.issuer,
        validFrom: draft.valid_from ?? prev.validFrom,
        validUntil: draft.valid_until ?? prev.validUntil,
        indefinite: draft.valid_until ? false : prev.indefinite,
        documentTypeCode: draft.document_type_code ?? prev.documentTypeCode,
        offsets: draft.reminder_offsets_days.length > 0 ? [...draft.reminder_offsets_days] : prev.offsets,
      }));
      setErrors({});
      setNotice(
        draft.confidence >= 0.5
          ? 'Поля заполнены по тексту — проверьте их перед сохранением'
          : 'Распознано не всё: проверьте и дополните поля вручную',
      );
    },
    onError: (error) => setNotice(messageForError(error)),
  });

  const save = useMutation({
    mutationFn: async () => {
      if (mode === 'create') return createDocument(organizationId, toCreateBody(newId.current, form));
      const version = existing.data?.version ?? 0;
      return updateDocument(docId, toUpdateBody(version, form));
    },
    onSuccess: async (document) => {
      saved.current = true;
      setInitial(form);
      await setClosingConfirmation(false);
      void haptic('success');
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization(organizationId) });
      await queryClient.invalidateQueries({ queryKey: queryKeys.suggestions(organizationId) });
      queryClient.setQueryData(queryKeys.document(document.id), document);
      navigate(`/d/${document.id}`, { replace: true });
    },
    onError: async (error) => {
      void haptic('error');
      if (error instanceof ApiError) {
        const fields = error.fieldErrors();
        setErrors({
          ...(fields.title ? { title: fields.title } : {}),
          ...(fields.number ? { number: fields.number } : {}),
          ...(fields.issuer ? { issuer: fields.issuer } : {}),
          ...(fields.responsible_label ? { responsibleLabel: fields.responsible_label } : {}),
          ...(fields.notes ? { notes: fields.notes } : {}),
          ...(fields.reference_url ? { referenceUrl: fields.reference_url } : {}),
          ...(fields.valid_until ? { validUntil: fields.valid_until } : {}),
          ...(fields.reminder_offsets_days ? { offsets: fields.reminder_offsets_days } : {}),
          form: messageForError(error),
        });
        if (error.code === 'CONFLICT_ID_REUSED') {
          newId.current = uuidV4();
          setNotice('Идентификатор уже использован — повторите сохранение.');
        }
        if (error.code === 'CONFLICT_VERSION' && mode === 'edit') {
          await existing.refetch();
          setNotice('Документ изменён другим участником: форма обновлена, проверьте значения.');
        }
        return;
      }
      setErrors({ form: messageForError(error) });
    },
  });

  const submit = () => {
    if (submitting.current || saved.current || save.isPending) return;
    const validation = validateDocumentForm(form);
    setErrors(validation);
    if (Object.keys(validation).length > 0) return;
    submitting.current = true;
    save.mutate(undefined, { onSettled: () => { submitting.current = false; } });
  };

  if (mode === 'edit' && existing.isLoading) {
    return (
      <AppShell title="Документ">
        <LoadingView rows={5} />
      </AppShell>
    );
  }
  if (mode === 'edit' && (existing.isError || !existing.data)) {
    return (
      <AppShell title="Документ">
        <ErrorView error={existing.error} onRetry={() => void existing.refetch()} />
      </AppShell>
    );
  }

  const types = catalog.data?.document_types ?? [];
  const selectedType = types.find((type) => type.code === form.documentTypeCode);

  return (
    <AppShell
      title={mode === 'create' ? 'Новый документ' : 'Изменить документ'}
      actions={
        <Button size="large" stretched loading={save.isPending} disabled={save.isPending} onClick={submit}>
          Сохранить
        </Button>
      }
    >
      {mode === 'create' && me.assistant_enabled ? (
        <section className="card stack stack--tight" aria-label="Быстрый ввод">
          <h2 className="card__title">Быстрый ввод</h2>
          <p className="card__text">
            Вставьте строку из таблицы или текст документа — поля формы заполнятся автоматически.
          </p>
          <Textarea
            aria-label="Текст документа для распознавания"
            value={assistantText}
            placeholder="Лицензия на алкоголь № 78РПА0012345, выдана 14.03.2024, действует до 13.03.2029"
            onChange={(event) => setAssistantText(event.target.value)}
          />
          <Button
            size="medium"
            variant="secondary"
            loading={recognize.isPending}
            disabled={recognize.isPending || assistantText.trim() === ''}
            onClick={() => recognize.mutate()}
          >
            Заполнить по тексту
          </Button>
          <span className="field__hint">
            Текст обрабатывается сервисом GigaChat. Документ сохраняется только после вашего подтверждения.
          </span>
        </section>
      ) : null}

      {mode === 'create' ? (
        <div className="field">
          <label className="field__label" htmlFor="document-type">
            Тип документа
          </label>
          <select
            id="document-type"
            value={form.documentTypeCode ?? ''}
            onChange={(event) => {
              const code = event.target.value === '' ? null : event.target.value;
              const type = types.find((item) => item.code === code);
              setForm((prev) => ({
                ...prev,
                documentTypeCode: code,
                title: prev.title === '' && type ? type.title : prev.title,
                offsets:
                  type && type.default_reminder_offsets_days.length > 0
                    ? [...type.default_reminder_offsets_days]
                    : prev.offsets,
              }));
            }}
          >
            <option value="">Свой документ</option>
            {types.map((type) => (
              <option key={type.code} value={type.code}>
                {type.title}
              </option>
            ))}
          </select>
          {selectedType?.data_status === 'model' ? <ModelDataBadge compact /> : null}
        </div>
      ) : null}

      <div className="field">
        <label className="field__label" htmlFor="document-title">
          Название
        </label>
        <Input
          id="document-title"
          value={form.title}
          aria-invalid={errors.title ? true : undefined}
          onChange={(event) => setForm((prev) => ({ ...prev, title: event.target.value }))}
        />
        {errors.title ? (
          <span className="field__error" role="alert">
            {errors.title}
          </span>
        ) : null}
      </div>

      <div className="field">
        <label className="field__label" htmlFor="document-number">
          Номер
        </label>
        <Input
          id="document-number"
          value={form.number}
          onChange={(event) => setForm((prev) => ({ ...prev, number: event.target.value }))}
        />
        {errors.number ? (
          <span className="field__error" role="alert">
            {errors.number}
          </span>
        ) : null}
      </div>

      <div className="field">
        <label className="field__label" htmlFor="document-issuer">
          Кем выдан
        </label>
        <Input
          id="document-issuer"
          value={form.issuer}
          onChange={(event) => setForm((prev) => ({ ...prev, issuer: event.target.value }))}
        />
      </div>

      <div className="field">
        <label className="field__label" htmlFor="document-responsible">
          Ответственный
        </label>
        <Input
          id="document-responsible"
          value={form.responsibleLabel}
          onChange={(event) => setForm((prev) => ({ ...prev, responsibleLabel: event.target.value }))}
        />
        <span className="field__hint">Должность, без ФИО</span>
      </div>

      <DateField
        label="Действует с"
        value={form.validFrom}
        onChange={(value) => setForm((prev) => ({ ...prev, validFrom: value }))}
      />

      <div className="field">
        <div className="row row--between">
          <span className="field__label">Бессрочный документ</span>
          <Switch
            checked={form.indefinite}
            aria-label="Бессрочный документ"
            onChange={(event) =>
              setForm((prev) => ({
                ...prev,
                indefinite: event.target.checked,
                validUntil: event.target.checked ? null : prev.validUntil,
              }))
            }
          />
        </div>
      </div>

      {!form.indefinite ? (
        <DateField
          label="Действует до"
          value={form.validUntil}
          error={errors.validUntil}
          min={form.validFrom ?? undefined}
          onChange={(value) => setForm((prev) => ({ ...prev, validUntil: value }))}
        />
      ) : null}

      <OffsetChips
        value={form.offsets}
        error={errors.offsets}
        onChange={(offsets) => setForm((prev) => ({ ...prev, offsets }))}
      />

      <div className="field">
        <label className="field__label" htmlFor="document-reference">
          Ссылка на источник
        </label>
        <Input
          id="document-reference"
          value={form.referenceUrl}
          placeholder="https://"
          onChange={(event) => setForm((prev) => ({ ...prev, referenceUrl: event.target.value }))}
        />
        {errors.referenceUrl ? (
          <span className="field__error" role="alert">
            {errors.referenceUrl}
          </span>
        ) : null}
        {canScan ? (
          <Button
            size="medium"
            variant="secondary"
            onClick={() => {
              void scanCode().then((result) => {
                if (result.kind === 'url') setForm((prev) => ({ ...prev, referenceUrl: result.value }));
                else if (result.kind === 'number') setForm((prev) => ({ ...prev, number: result.value }));
                else setNotice('Код не похож на ссылку или номер документа');
              });
            }}
          >
            Сканировать QR
          </Button>
        ) : null}
      </div>

      <div className="field">
        <label className="field__label" htmlFor="document-notes">
          Заметки
        </label>
        <Textarea
          id="document-notes"
          value={form.notes}
          onChange={(event) => setForm((prev) => ({ ...prev, notes: event.target.value }))}
        />
        {errors.notes ? (
          <span className="field__error" role="alert">
            {errors.notes}
          </span>
        ) : null}
      </div>

      {notice ? (
        <p className="banner" aria-live="polite">
          {notice}
        </p>
      ) : null}
      {errors.form ? (
        <p className="field__error" role="alert" aria-live="polite">
          {errors.form}
        </p>
      ) : null}
    </AppShell>
  );
}
