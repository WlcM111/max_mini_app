import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate, useNavigationType, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { TextField } from '../../shared/ui/Field';
import { Icon } from '../../shared/ui/Icon';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { Steps } from '../../shared/ui/Steps';
import { uuidV4 } from '../../shared/lib/uuid';
import { getLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
import { listSuggestions } from '../organizations/api';
import { clearPick, emptyPick, readPick, validateCustomTitle, writePick, type DocumentPick, type PickScope } from './documentPick';
import { getDraft, resetDraft, updateDraft } from './onboardingDraft';

/**
 * Подбор типовых документов по профилю (FR-04) и свои документы (FR-27).
 * Шаг 3 онбординга или экран раздела «Документы» (маршрут с :orgId).
 */
export function SuggestionsPage() {
  const navigate = useNavigate();
  const navigationType = useNavigationType();
  const { orgId } = useParams();
  const { me } = useSession();
  const inOrganization = Boolean(orgId);
  const organizationId = orgId ?? getLastOrganization() ?? me.memberships[0]?.organization_id ?? getDraft().organizationId;
  const organizationName = me.memberships.find((item) => item.organization_id === organizationId)?.organization_name;
  const scope = useMemo<PickScope>(() => (orgId ? { kind: 'organization', organizationId: orgId } : { kind: 'onboarding' }), [orgId]);
  // Новый вход в раздел начинает выбор заново; возврат «Назад» со шага сроков его сохраняет.
  const [pick, setPick] = useState<DocumentPick>(() => (orgId && navigationType === 'PUSH' ? emptyPick() : readPick(scope)));
  const [customTitle, setCustomTitle] = useState('');
  const [customError, setCustomError] = useState<string | null>(null);

  useEffect(() => writePick(scope, pick), [scope, pick]);

  const suggestions = useQuery({
    queryKey: queryKeys.suggestions(organizationId),
    queryFn: () => listSuggestions(organizationId),
    enabled: organizationId !== '',
  });
  const items = suggestions.data ?? [];
  const selected = pick.selectedTypes;
  const allSelected = items.length > 0 && items.every((item) => selected.includes(item.document_type_code));
  const total = selected.length + pick.customDocuments.length;

  const toggle = (code: string) =>
    setPick((prev) => ({
      ...prev,
      selectedTypes: prev.selectedTypes.includes(code) ? prev.selectedTypes.filter((item) => item !== code) : [...prev.selectedTypes, code],
    }));
  const toggleAll = () => setPick((prev) => ({ ...prev, selectedTypes: allSelected ? [] : items.map((item) => item.document_type_code) }));

  const addCustom = () => {
    const taken = [
      ...pick.customDocuments.map((item) => item.title),
      ...items.filter((item) => selected.includes(item.document_type_code)).map((item) => item.title),
    ];
    const invalid = validateCustomTitle(customTitle, taken);
    if (invalid) {
      setCustomError(invalid);
      return;
    }
    setPick((prev) => ({ ...prev, customDocuments: [...prev.customDocuments, { id: uuidV4(), title: customTitle.trim() }] }));
    setCustomTitle('');
    setCustomError(null);
  };
  const removeCustom = (id: string) =>
    setPick((prev) => ({ ...prev, customDocuments: prev.customDocuments.filter((item) => item.id !== id) }));

  // Выход без выбора: черновик онбординга сбрасывается, чтобы вторая организация получила новый id.
  const finish = () => {
    if (inOrganization) {
      clearPick(scope);
      navigate(-1);
      return;
    }
    resetDraft();
    navigate(`/o/${organizationId}`, { replace: true });
  };
  const next = () => {
    if (total === 0) {
      finish();
      return;
    }
    if (inOrganization) {
      navigate(`/o/${organizationId}/documents/typical/dates`);
      return;
    }
    updateDraft({ organizationId });
    navigate('/onboarding/dates');
  };

  return (
    <AppShell
      layout={inOrganization ? 'stack' : 'split'}
      title="Типовые документы"
      subtitle={inOrganization ? organizationName : 'Отметьте документы, которые есть у организации, — сроки укажете на следующем шаге.'}
      headerExtra={inOrganization ? null : <Steps current={3} total={4} />}
      actions={
        <>
          <Button size="l" stretched iconRight={total > 0 ? 'arrow-right' : undefined} onClick={next}>
            {total > 0 ? `Далее (${total})` : inOrganization ? 'Готово' : 'Пропустить'}
          </Button>
          {!inOrganization ? (
            <Button variant="tertiary" stretched onClick={finish}>
              Добавлю позже
            </Button>
          ) : null}
        </>
      }
    >
      {suggestions.isLoading ? <LoadingView rows={4} /> : null}
      {suggestions.isError ? <ErrorView error={suggestions.error} onRetry={() => void suggestions.refetch()} /> : null}
      {suggestions.isSuccess && items.length === 0 ? (
        <EmptyView
          art="none"
          title="Все типовые документы уже добавлены"
          description="Новые подсказки появятся, если изменить профиль организации. Свой документ можно добавить ниже."
        />
      ) : null}
      {items.length > 0 ? (
        <>
          <div className="select-bar">
            <span>
              Выбрано {selected.length} из {items.length}
            </span>
            <Button variant="tertiary" size="s" onClick={toggleAll}>
              {allSelected ? 'Снять все' : 'Выбрать все'}
            </Button>
          </div>
          <div className="list stagger">
            {items.map((item) => {
              const on = selected.includes(item.document_type_code);
              return (
                <button key={item.document_type_code} type="button" className="pick" aria-pressed={on} onClick={() => toggle(item.document_type_code)}>
                  <span className="pick__check" aria-hidden="true">
                    <Icon name="check" size={16} />
                  </span>
                  <span className="pick__body">
                    <span className="pick__title">{item.title}</span>
                    {item.description ? <span className="pick__text">{item.description}</span> : null}
                  </span>
                </button>
              );
            })}
          </div>
          <ModelDataBadge compact />
        </>
      ) : null}

      {suggestions.isSuccess ? (
        <section className="group" aria-labelledby="custom-docs-title">
          <h2 className="group__title" id="custom-docs-title">
            Свои документы
          </h2>
          {pick.customDocuments.length > 0 ? (
            <div className="list">
              {pick.customDocuments.map((item) => (
                <div key={item.id} className="pick pick--custom">
                  <span className="pick__check" aria-hidden="true">
                    <Icon name="check" size={16} />
                  </span>
                  <span className="pick__body">
                    <span className="pick__title">{item.title}</span>
                  </span>
                  <button type="button" className="icon-btn" aria-label={`Убрать «${item.title}»`} onClick={() => removeCustom(item.id)}>
                    <Icon name="x" size={20} />
                  </button>
                </div>
              ))}
            </div>
          ) : null}
          <div className="custom-add">
            <TextField
              id="custom-document-title"
              label="Название своего документа"
              labelHidden
              value={customTitle}
              placeholder="Например, договор с поставщиком"
              maxLength={200}
              error={customError}
              onChange={(value) => {
                setCustomTitle(value);
                setCustomError(null);
              }}
              onEnter={addCustom}
            />
            <Button variant="secondary" size="l" icon="plus" onClick={addCustom}>
              Добавить
            </Button>
          </div>
          <p className="group__foot">Любой документ со сроком, которого нет в списке: договор, разрешение, сертификат. Срок укажете на следующем шаге.</p>
        </section>
      ) : null}
    </AppShell>
  );
}
