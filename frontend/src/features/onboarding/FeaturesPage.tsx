import { useState } from 'react';
import { Button, Switch } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { setLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
import { uuidV4 } from '../../shared/lib/uuid';
import { createOrganization, getCatalog } from '../organizations/api';
import { getDraft, updateDraft } from './onboardingDraft';

/** Шаг 2: признаки бизнеса; по завершении создаётся организация (FR-04). */
export function FeaturesPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { refreshMe } = useSession();
  const draft = getDraft();
  const [selected, setSelected] = useState<string[]>(draft.featureCodes);
  const [error, setError] = useState<string | null>(null);

  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });

  const create = useMutation({
    mutationFn: () => {
      const current = updateDraft({ featureCodes: selected });
      return createOrganization({
        id: current.organizationId,
        name: current.name,
        business_category_code: current.businessCategoryCode,
        region_code: current.regionCode,
        timezone: current.timezone,
        feature_codes: current.featureCodes,
      });
    },
    onSuccess: async (organization) => {
      setLastOrganization(organization.id);
      queryClient.setQueryData(queryKeys.organization(organization.id), organization);
      await refreshMe();
      navigate('/onboarding/suggestions', { replace: true });
    },
    onError: (failure) => {
      if (failure instanceof ApiError && failure.code === 'CONFLICT_ID_REUSED') {
        updateDraft({ organizationId: uuidV4() });
        setError('Повторите сохранение: идентификатор уже использован.');
        return;
      }
      setError(messageForError(failure));
    },
  });

  if (catalog.isLoading) {
    return (
      <AppShell title="Что относится к вам">
        <LoadingView rows={4} />
      </AppShell>
    );
  }
  if (catalog.isError || !catalog.data) {
    return (
      <AppShell title="Что относится к вам">
        <ErrorView error={catalog.error} onRetry={() => void catalog.refetch()} />
      </AppShell>
    );
  }

  return (
    <AppShell
      title="Что относится к вам"
      subtitle="Шаг 2 из 4"
      actions={
        <Button size="large" stretched loading={create.isPending} onClick={() => create.mutate()}>
          Создать организацию
        </Button>
      }
    >
      <p className="card__text">Ответы помогут подобрать документы, которые обычно нужны такому бизнесу.</p>
      {catalog.data.features.map((feature) => (
        <div key={feature.code} className="card row row--between">
          <span className="grow">
            <span className="list-item__title">{feature.question}</span>
            {feature.hint ? <span className="list-item__meta">{feature.hint}</span> : null}
          </span>
          <Switch
            checked={selected.includes(feature.code)}
            aria-label={feature.question}
            onChange={(event) =>
              setSelected((prev) =>
                event.target.checked ? [...prev, feature.code] : prev.filter((code) => code !== feature.code),
              )
            }
          />
        </div>
      ))}
      {error ? (
        <p className="field__error" role="alert" aria-live="polite">
          {error}
        </p>
      ) : null}
    </AppShell>
  );
}
