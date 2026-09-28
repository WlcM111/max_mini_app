import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { DateField } from '../../shared/ui/DateField';
import { SelectField, TextAreaField, TextField } from '../../shared/ui/Field';
import { Icon } from '../../shared/ui/Icon';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { OffsetChips } from '../../shared/ui/OffsetChips';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { ToggleRow } from '../../shared/ui/Switch';
import { toast } from '../../shared/ui/Toast';
import { uuidV4 } from '../../shared/lib/uuid';
import { getBridge } from '../../platform/max/bridge';
import { scanCode } from '../../platform/max/codeReader';
import { haptic } from '../../platform/max/haptics';
import { setClosingConfirmation } from '../../platform/max/closingConfirmation';
import { useSession } from '../../session/useSession';
import { getCatalog } from '../organizations/api';
import type { DocumentDraft } from '../../api/client';
import { prepareImage } from '../../shared/lib/image';
import { cx } from '../../shared/lib/cx';
import { createDocument, draftDocument, draftDocumentFromImage, getDocument, updateDocument } from './api';
import { listMembers } from '../members/api';
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

type FlashKey = keyof DocumentFormState;

/** Форма документа: создание и изменение с защитой от повторной отправки. */
export function DocumentFormPage({ mode }: Props) {
  const { orgId = '', docId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  // Идентификатор создаётся один раз при открытии формы: повтор отправки
  // не создаёт дубль (идемпотентность по клиентскому UUID).
  const newId = useRef(uuidV4());
  // Синхронный замок: защищает от двойного нажатия до того, как мутация станет pending.
  const submitting = useRef(false);
  // После успешного сохранения форма больше не отправляется.
  const saved = useRef(false);
  const [form, setForm] = useState<DocumentFormState>(() => emptyDocumentForm());
  const [initial, setInitial] = useState<DocumentFormState>(() => emptyDocumentForm());
  const [errors, setErrors] = useState<FormErrors>({});
  const [focusError, setFocusError] = useState(0);
  const [canScan, setCanScan] = useState(false);
  const [assistNotice, setAssistNotice] = useState<string | null>(null);
  const [assistantText, setAssistantText] = useState('');
  const [flash, setFlash] = useState<FlashKey[]>([]);
  const [photo, setPhoto] = useState<string | null>(null);
  const { me } = useSession();

  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });
  const existing = useQuery({
    queryKey: queryKeys.document(docId),
    queryFn: () => getDocument(docId),
    enabled: mode === 'edit' && docId !== '',
  });
  const organizationId = mode === 'edit' ? (existing.data?.organization_id ?? '') : orgId;
  const members = useQuery({
    queryKey: queryKeys.members(organizationId),
    queryFn: () => listMembers(organizationId),
    enabled: organizationId !== '',
    staleTime: 60_000,
  });

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

  // Подсветка полей, заполненных по тексту, гаснет сама.
  useEffect(() => {
    if (flash.length === 0) return undefined;
    const timer = window.setTimeout(() => setFlash([]), 1500);
    return () => window.clearTimeout(timer);
  }, [flash]);

  // После неудачной проверки — фокус на первое поле с ошибкой.
  useEffect(() => {
    if (focusError === 0) return;
    const field = window.document.querySelector<HTMLElement>('[aria-invalid="true"]');
    if (!field) return;
    field.focus({ preventScroll: true });
    field.scrollIntoView?.({ block: 'center', behavior: 'smooth' });
  }, [focusError]);

  const update = <K extends keyof DocumentFormState>(key: K, value: DocumentFormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  // Быстрый ввод: ассистент распознаёт реквизиты по тексту или фото и заполняет форму (FR-21).
  // Ничего не сохраняет — пользователь проверяет поля и нажимает «Сохранить».
  const applyDraft = (draft: DocumentDraft, source: 'text' | 'photo') => {
    const filled: FlashKey[] = [];
    setForm((prev) => {
      const next: DocumentFormState = {
        ...prev,
        title: draft.title !== '' ? draft.title : prev.title,
        number: draft.number ?? prev.number,
        issuer: draft.issuer ?? prev.issuer,
        validFrom: draft.valid_from ?? prev.validFrom,
        validUntil: draft.valid_until ?? prev.validUntil,
        indefinite: draft.valid_until ? false : prev.indefinite,
        documentTypeCode: draft.document_type_code ?? prev.documentTypeCode,
        offsets: draft.reminder_offsets_days.length > 0 ? [...draft.reminder_offsets_days] : prev.offsets,
      };
      (['title', 'number', 'issuer', 'validFrom', 'validUntil', 'documentTypeCode'] as const).forEach((key) => {
        if (next[key] !== prev[key]) filled.push(key);
      });
      return next;
    });
    setFlash(filled);
    setErrors({});
    setAssistNotice(
      draft.confidence >= 0.5
        ? `Поля заполнены по ${source === 'text' ? 'тексту' : 'фото'} — проверьте их перед сохранением`
        : 'Распознано не всё: проверьте и дополните поля вручную',
    );
  };

  const recognize = useMutation({
    mutationFn: () => draftDocument(organizationId, assistantText),
    onSuccess: (draft) => applyDraft(draft, 'text'),
    onError: (error) => setAssistNotice(messageForError(error)),
  });

  const recognizePhoto = useMutation({
    mutationFn: async (file: File) => {
      const prepared = await prepareImage(file);
      setPhoto(prepared.dataUrl);
      return draftDocumentFromImage(organizationId, prepared.base64, 'image/jpeg');
    },
    onSuccess: (draft) => applyDraft(draft, 'photo'),
    onError: (error) => setAssistNotice(error instanceof ApiError || !(error instanceof Error) ? messageForError(error) : 'Не удалось прочитать фото — попробуйте ещё раз'),
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
      toast(mode === 'create' ? 'Документ добавлен' : 'Изменения сохранены', 'success');
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
        setFocusError((value) => value + 1);
        if (error.code === 'CONFLICT_ID_REUSED') {
          newId.current = uuidV4();
          toast('Идентификатор уже использован — повторите сохранение.');
        }
        if (error.code === 'CONFLICT_VERSION' && mode === 'edit') {
          await existing.refetch();
          toast('Документ изменён другим участником: форма обновлена, проверьте значения.');
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
    if (Object.keys(validation).length > 0) {
      setFocusError((value) => value + 1);
      void haptic('error');
      return;
    }
    submitting.current = true;
    save.mutate(undefined, {
      onSettled: () => {
        submitting.current = false;
      },
    });
  };

  const scan = () => {
    void scanCode().then((result) => {
      if (result.kind === 'url') {
        update('referenceUrl', result.value);
        setFlash(['referenceUrl']);
      } else if (result.kind === 'number') {
        update('number', result.value);
        setFlash(['number']);
      } else toast('Код не похож на ссылку или номер документа');
    });
  };

  const title = mode === 'create' ? 'Новый документ' : 'Изменить документ';

  if (mode === 'edit' && existing.isLoading) {
    return (
      <AppShell title={title}>
        <LoadingView rows={5} variant="form" />
      </AppShell>
    );
  }

  if (mode === 'edit' && (existing.isError || !existing.data)) {
    return (
      <AppShell title={title}>
        <ErrorView error={existing.error} onRetry={() => void existing.refetch()} />
      </AppShell>
    );
  }

  const types = catalog.data?.document_types ?? [];
  const selectedType = types.find((type) => type.code === form.documentTypeCode);
  const isFlash = (key: FlashKey) => flash.includes(key);

  return (
    <AppShell
      title={title}
      subtitle={mode === 'edit' ? existing.data?.title : undefined}
      actionsNote={
        errors.form ? (
          <p className="actionbar__note" role="alert" aria-live="polite">
            <Icon name="alert" size={18} />
            {errors.form}
          </p>
        ) : null
      }
      actions={
        <Button size="l" stretched loading={save.isPending} onClick={submit}>
          Сохранить
        </Button>
      }
    >
      {mode === 'create' && me.assistant_enabled ? (
        <section className="assist" aria-labelledby="assist-title">
          <div className="assist__head">
            <span className="assist__icon" aria-hidden="true">
              <Icon name="sparkles" />
            </span>
            <div>
              <h2 className="assist__title" id="assist-title">
                Быстрый ввод
              </h2>
              <p className="assist__text">Сфотографируйте документ или вставьте его текст — поля заполнятся автоматически.</p>
            </div>
          </div>
          <TextAreaField
            label="Текст документа для распознавания"
            labelHidden
            value={assistantText}
            onChange={setAssistantText}
            rows={3}
            placeholder="Лицензия на алкоголь № 78РПА0012345, выдана 14.03.2024, действует до 13.03.2029"
          />
          <div className="assist__actions">
            <Button
              variant="secondary"
              icon="sparkles"
              loading={recognize.isPending}
              disabled={assistantText.trim() === '' || recognizePhoto.isPending}
              onClick={() => recognize.mutate()}
            >
              Заполнить по тексту
            </Button>
            <label className={cx('btn', 'btn--neutral', 'btn--m', 'assist__photo-btn', recognizePhoto.isPending && 'is-loading')}>
              {recognizePhoto.isPending ? <span className="btn__spinner" aria-hidden="true" /> : <Icon name="camera" />}
              <span className="btn__label">{recognizePhoto.isPending ? 'Читаем фото…' : 'Фото документа'}</span>
              <input
                type="file"
                accept="image/*"
                capture="environment"
                className="visually-hidden"
                disabled={recognizePhoto.isPending}
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  if (file) recognizePhoto.mutate(file);
                  event.target.value = '';
                }}
              />
            </label>
          </div>
          {assistNotice ? (
            <p className="assist__notice" aria-live="polite">
              {assistNotice}
            </p>
          ) : null}
          {photo ? <img className="assist__photo" src={photo} alt="Фотография документа" /> : null}
          <p className="assist__fine">
            Текст и фото обрабатывает сервис GigaChat, фото сразу удаляется. Документ сохраняется только после вашего подтверждения.
          </p>
        </section>
      ) : null}

      <section className="group" aria-labelledby="doc-main-title">
        <h2 className="group__title" id="doc-main-title">
          Документ
        </h2>
        <div className="form-card">
          {mode === 'create' ? (
            <div className="field">
              <SelectField
                id="document-type"
                label="Тип документа"
                value={form.documentTypeCode ?? ''}
                flash={isFlash('documentTypeCode')}
                hint={selectedType?.description ?? 'Тип подскажет срок напоминаний и шаги продления'}
                options={[{ value: '', label: 'Свой документ' }, ...types.map((type) => ({ value: type.code, label: type.title }))]}
                onChange={(value) => {
                  const code = value === '' ? null : value;
                  const type = types.find((item) => item.code === code);
                  setForm((prev) => ({
                    ...prev,
                    documentTypeCode: code,
                    title: prev.title === '' && type ? type.title : prev.title,
                    offsets: type && type.default_reminder_offsets_days.length > 0 ? [...type.default_reminder_offsets_days] : prev.offsets,
                  }));
                }}
              />
              {selectedType?.data_status === 'model' ? <ModelDataBadge compact /> : null}
            </div>
          ) : null}
          <TextField
            id="document-title"
            label="Название"
            value={form.title}
            error={errors.title}
            flash={isFlash('title')}
            maxLength={200}
            onChange={(value) => update('title', value)}
          />
          <TextField
            id="document-number"
            label="Номер"
            value={form.number}
            error={errors.number}
            flash={isFlash('number')}
            onChange={(value) => update('number', value)}
          />
          <TextField
            id="document-issuer"
            label="Кем выдан"
            value={form.issuer}
            error={errors.issuer}
            flash={isFlash('issuer')}
            onChange={(value) => update('issuer', value)}
          />
          <SelectField
            id="document-responsible-member"
            label="Ответственный"
            hint={form.responsibleAccountId ? 'Напоминания будут приходить лично ему' : 'Не назначен — напоминания получают все участники'}
            value={form.responsibleAccountId ?? ''}
            options={[
              { value: '', label: 'Не назначен' },
              ...(members.data ?? []).map((member) => ({
                value: member.account_id,
                label: `${[member.first_name, member.last_name].filter(Boolean).join(' ')}${member.is_me ? ' (вы)' : ''}`,
              })),
            ]}
            onChange={(value) => update('responsibleAccountId', value === '' ? null : value)}
          />
          <TextField
            id="document-responsible"
            label="Должность ответственного"
            hint="Если его нет в команде — укажите должность, без ФИО"
            value={form.responsibleLabel}
            error={errors.responsibleLabel}
            onChange={(value) => update('responsibleLabel', value)}
          />
        </div>
      </section>

      <section className="group" aria-labelledby="doc-dates-title">
        <h2 className="group__title" id="doc-dates-title">
          Срок действия
        </h2>
        <div className="form-card">
          <DateField label="Действует с" value={form.validFrom} flash={isFlash('validFrom')} onChange={(value) => update('validFrom', value)} />
          <ToggleRow
            flush
            title="Бессрочный документ"
            hint="Без даты окончания — напоминания не нужны"
            checked={form.indefinite}
            onChange={(checked) =>
              setForm((prev) => ({ ...prev, indefinite: checked, validUntil: checked ? null : prev.validUntil }))
            }
          />
          {!form.indefinite ? (
            <div className="reveal">
              <DateField
                label="Действует до"
                value={form.validUntil}
                error={errors.validUntil}
                min={form.validFrom ?? undefined}
                flash={isFlash('validUntil')}
                onChange={(value) => update('validUntil', value)}
              />
            </div>
          ) : null}
        </div>
      </section>

      {!form.indefinite ? (
        <section className="group reveal" aria-labelledby="doc-offsets-title">
          <h2 className="group__title" id="doc-offsets-title">
            Напоминания
          </h2>
          <div className="form-card">
            <OffsetChips value={form.offsets} error={errors.offsets} onChange={(offsets) => update('offsets', offsets)} />
          </div>
        </section>
      ) : null}

      <section className="group" aria-labelledby="doc-extra-title">
        <h2 className="group__title" id="doc-extra-title">
          Дополнительно
        </h2>
        <div className="form-card">
          <TextField
            id="document-reference"
            label="Ссылка на источник"
            type="url"
            inputMode="url"
            placeholder="https://"
            value={form.referenceUrl}
            error={errors.referenceUrl}
            flash={isFlash('referenceUrl')}
            onChange={(value) => update('referenceUrl', value)}
            trailing={
              canScan ? (
                <button type="button" className="control__action control__action--accent" aria-label="Сканировать QR" onClick={scan}>
                  <Icon name="qr" />
                </button>
              ) : null
            }
          />
          <TextAreaField id="document-notes" label="Заметки" value={form.notes} error={errors.notes} onChange={(value) => update('notes', value)} />
        </div>
      </section>
    </AppShell>
  );
}
