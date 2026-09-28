import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { SelectField, TextField } from '../../shared/ui/Field';
import { Icon } from '../../shared/ui/Icon';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { ToggleRow } from '../../shared/ui/Switch';
import { toast } from '../../shared/ui/Toast';
import { timezoneLabel } from '../../shared/lib/format';
import { validateOrganizationName } from '../../shared/lib/validation';
import { getCatalog, getOrganization, updateOrganization } from './api';

/** Профиль организации: название, вид деятельности, регион, пояс, признаки. */
export function OrganizationSettingsPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [categoryCode, setCategoryCode] = useState('');
  const [regionCode, setRegionCode] = useState('');
  const [timezone, setTimezone] = useState('');
  const [features, setFeatures] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [nameError, setNameError] = useState<string | null>(null);

  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });
  const organization = useQuery({
    queryKey: queryKeys.organization(orgId),
    queryFn: () => getOrganization(orgId),
    enabled: orgId !== '',
  });

  useEffect(() => {
    const data = organization.data;
    if (!data) return;
    setName(data.name);
    setCategoryCode(data.business_category_code);
    setRegionCode(data.region_code);
    setTimezone(data.timezone);
    setFeatures([...data.feature_codes]);
  }, [organization.data]);

  const save = useMutation({
    mutationFn: () =>
      updateOrganization(orgId, {
        expected_version: organization.data?.version ?? 0,
        name: name.trim(),
        business_category_code: categoryCode,
        region_code: regionCode,
        timezone,
        feature_codes: features,
      }),
    onSuccess: async (updated) => {
      queryClient.setQueryData(queryKeys.organization(orgId), updated);
      await queryClient.invalidateQueries({ queryKey: queryKeys.suggestions(orgId) });
      await queryClient.invalidateQueries({ queryKey: queryKeys.me() });
      toast('Профиль сохранён', 'success');
      navigate(`/o/${orgId}/settings`, { replace: true });
    },
    onError: async (failure) => {
      if (failure instanceof ApiError && failure.code === 'CONFLICT_VERSION') {
        await organization.refetch();
        setError('Профиль изменён другим участником: данные обновлены, проверьте значения.');
        return;
      }
      setError(messageForError(failure));
    },
  });

  if (organization.isLoading || catalog.isLoading) {
    return (
      <AppShell title="Профиль организации">
        <LoadingView rows={4} variant="form" />
      </AppShell>
    );
  }

  if (organization.isError || !organization.data || !catalog.data) {
    return (
      <AppShell title="Профиль организации">
        <ErrorView error={organization.error ?? catalog.error} onRetry={() => void organization.refetch()} />
      </AppShell>
    );
  }

  const data = catalog.data;
  const timezones = Array.from(new Set([...data.regions.map((region) => region.default_timezone), timezone].filter(Boolean)));

  const submit = () => {
    const invalid = validateOrganizationName(name);
    setNameError(invalid);
    if (invalid) return;
    setError(null);
    save.mutate();
  };

  return (
    <AppShell
      title="Профиль организации"
      subtitle="От профиля зависит подбор типовых документов"
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
          Сохранить
        </Button>
      }
    >
      <section className="group" aria-labelledby="org-main-title">
        <h2 className="group__title" id="org-main-title">
          Основное
        </h2>
        <div className="form-card">
          <TextField id="org-name" label="Название" value={name} error={nameError} maxLength={200} onChange={setName} />
          <SelectField
            id="org-category"
            label="Вид деятельности"
            value={categoryCode}
            onChange={setCategoryCode}
            options={data.business_categories.map((item) => ({ value: item.code, label: item.title }))}
          />
          <SelectField
            id="org-region"
            label="Регион"
            value={regionCode}
            onChange={(code) => {
              setRegionCode(code);
              const region = data.regions.find((item) => item.code === code);
              if (region) setTimezone(region.default_timezone);
            }}
            options={data.regions.map((item) => ({ value: item.code, label: item.title }))}
          />
          <SelectField
            id="org-timezone"
            label="Часовой пояс"
            hint="В этом поясе приходят напоминания"
            value={timezone}
            onChange={setTimezone}
            options={timezones.map((zone) => ({ value: zone, label: timezoneLabel(zone) }))}
          />
        </div>
      </section>
      {data.features.length > 0 ? (
        <section className="group" aria-labelledby="org-features-title">
          <h2 className="group__title" id="org-features-title">
            Особенности бизнеса
          </h2>
          <div className="group__card">
            {data.features.map((feature) => (
              <ToggleRow
                key={feature.code}
                title={feature.question}
                hint={feature.hint ?? undefined}
                checked={features.includes(feature.code)}
                onChange={(checked) =>
                  setFeatures((prev) => (checked ? [...prev, feature.code] : prev.filter((code) => code !== feature.code)))
                }
              />
            ))}
          </div>
        </section>
      ) : null}
    </AppShell>
  );
}
