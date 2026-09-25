import { useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { useSession } from '../../session/useSession';
import { getLastOrganization } from '../../session/sessionStore';
import { listSuggestions } from '../organizations/api';
import { getDraft, updateDraft } from './onboardingDraft';

/** Шаг 3: подбор типовых документов по профилю организации (FR-05). */
export function SuggestionsPage() {
  const navigate = useNavigate();
  const { me } = useSession();
  const draft = getDraft();
  const organizationId = getLastOrganization() ?? me.memberships[0]?.organization_id ?? draft.organizationId;
  const [selected, setSelected] = useState<string[]>(draft.selectedTypes);

  const suggestions = useQuery({
    queryKey: queryKeys.suggestions(organizationId),
    queryFn: () => listSuggestions(organizationId),
    enabled: organizationId !== '',
  });

  const toggle = (code: string) =>
    setSelected((prev) => (prev.includes(code) ? prev.filter((item) => item !== code) : [...prev, code]));

  const next = () => {
    updateDraft({ organizationId, selectedTypes: selected });
    if (selected.length === 0) {
      navigate(`/o/${organizationId}`, { replace: true });
      return;
    }
    navigate('/onboarding/dates');
  };

  return (
    <AppShell
      title="Типовые документы"
      subtitle="Шаг 3 из 4"
      actions={
        <>
          <Button size="large" stretched onClick={next}>
            {selected.length === 0 ? 'Пропустить' : `Далее (${selected.length})`}
          </Button>
          <Button
            size="medium"
            stretched
            variant="ghost"
            onClick={() => navigate(`/o/${organizationId}`, { replace: true })}
          >
            Добавлю позже
          </Button>
        </>
      }
    >
      <ModelDataBadge />
      {suggestions.isLoading ? <LoadingView rows={4} /> : null}
      {suggestions.isError ? <ErrorView error={suggestions.error} onRetry={() => void suggestions.refetch()} /> : null}
      {suggestions.data && suggestions.data.length === 0 ? (
        <EmptyView title="Все типовые документы уже добавлены" />
      ) : null}
      <div className="list">
        {suggestions.data?.map((item) => (
          <button
            key={item.document_type_code}
            type="button"
            className="list-item"
            aria-pressed={selected.includes(item.document_type_code)}
            onClick={() => toggle(item.document_type_code)}
          >
            <span className="grow">
              <span className="list-item__title">{item.title}</span>
              <span className="list-item__meta">{item.description}</span>
            </span>
            <span className={selected.includes(item.document_type_code) ? 'badge badge--valid' : 'badge badge--info'}>
              {selected.includes(item.document_type_code) ? 'Выбрано' : 'Добавить'}
            </span>
          </button>
        ))}
      </div>
    </AppShell>
  );
}
