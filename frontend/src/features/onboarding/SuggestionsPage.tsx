import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useLocation, useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { Steps } from '../../shared/ui/Steps';
import { getLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
import { listSuggestions } from '../organizations/api';
import { getDraft, resetDraft, updateDraft } from './onboardingDraft';

/** Шаг 3 (или вход с главной): подбор типовых документов по профилю (FR-05). */
export function SuggestionsPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { me } = useSession();
  const fromDashboard = (location.state as { from?: string } | null)?.from === 'dashboard';
  const draft = getDraft();
  const organizationId = getLastOrganization() ?? me.memberships[0]?.organization_id ?? draft.organizationId;
  const organizationName = me.memberships.find((item) => item.organization_id === organizationId)?.organization_name;
  const [selected, setSelected] = useState<string[]>(fromDashboard ? [] : draft.selectedTypes);

  const suggestions = useQuery({
    queryKey: queryKeys.suggestions(organizationId),
    queryFn: () => listSuggestions(organizationId),
    enabled: organizationId !== '',
  });
  const items = suggestions.data ?? [];
  const allSelected = items.length > 0 && items.every((item) => selected.includes(item.document_type_code));

  const toggle = (code: string) =>
    setSelected((prev) => (prev.includes(code) ? prev.filter((item) => item !== code) : [...prev, code]));
  const toggleAll = () => setSelected(allSelected ? [] : items.map((item) => item.document_type_code));

  // Выход без выбора: черновик онбординга сбрасывается, чтобы вторая организация получила новый id.
  const finish = () => {
    resetDraft();
    if (fromDashboard) navigate(-1);
    else navigate(`/o/${organizationId}`, { replace: true });
  };
  const next = () => {
    if (selected.length === 0) {
      finish();
      return;
    }
    updateDraft({ organizationId, selectedTypes: selected });
    navigate('/onboarding/dates', { state: { from: fromDashboard ? 'dashboard' : 'onboarding' } });
  };

  return (
    <AppShell
      title="Типовые документы"
      subtitle={fromDashboard ? organizationName : 'Отметьте документы, которые есть у организации, — сроки укажете на следующем шаге.'}
      headerExtra={fromDashboard ? null : <Steps current={3} total={4} />}
      actions={
        <>
          <Button size="l" stretched iconRight={selected.length > 0 ? 'arrow-right' : undefined} onClick={next}>
            {selected.length > 0 ? `Далее (${selected.length})` : fromDashboard ? 'Готово' : 'Пропустить'}
          </Button>
          {!fromDashboard ? (
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
          description="Новые подсказки появятся, если изменить профиль организации."
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
    </AppShell>
  );
}
