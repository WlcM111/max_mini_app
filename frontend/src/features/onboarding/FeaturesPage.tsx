import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { Steps } from '../../shared/ui/Steps';
import { ToggleRow } from '../../shared/ui/Switch';
import { uuidV4 } from '../../shared/lib/uuid';
import { setLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
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

  const steps = <Steps current={2} total={4} />;

  if (catalog.isLoading) {
    return (
      <AppShell title="Что относится к вам" headerExtra={steps}>
        <LoadingView rows={4} variant="form" />
      </AppShell>
    );
  }

  if (catalog.isError || !catalog.data) {
    return (
      <AppShell title="Что относится к вам" headerExtra={steps}>
        <ErrorView error={catalog.error} onRetry={() => void catalog.refetch()} />
      </AppShell>
    );
  }

  return (
    <AppShell
      layout="split"
      title="Что относится к вам"
      subtitle="От ответов зависит список нужных документов. Изменить их можно позже в профиле организации."
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
        <Button size="l" stretched loading={create.isPending} onClick={() => create.mutate()}>
          Создать организацию
        </Button>
      }
    >
      <div className="group__card">
        {catalog.data.features.map((feature) => (
          <ToggleRow
            key={feature.code}
            title={feature.question}
            hint={feature.hint ?? undefined}
            checked={selected.includes(feature.code)}
            onChange={(checked) =>
              setSelected((prev) => (checked ? [...prev, feature.code] : prev.filter((code) => code !== feature.code)))
            }
          />
        ))}
      </div>
    </AppShell>
  );
}
