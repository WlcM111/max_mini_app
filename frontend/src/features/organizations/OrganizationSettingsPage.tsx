import { useEffect, useState } from 'react';
import { Button, Input, Switch } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { ApiError, messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
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
        <LoadingView rows={4} />
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

  const submit = () => {
    const invalid = validateOrganizationName(name);
    if (invalid) {
      setError(invalid);
      return;
    }
    setError(null);
    save.mutate();
  };

  return (
    <AppShell
      title="Профиль организации"
      actions={
        <Button size="large" stretched loading={save.isPending} onClick={submit}>
          Сохранить
        </Button>
      }
    >
      <div className="field">
        <label className="field__label" htmlFor="org-name">
          Название
        </label>
        <Input id="org-name" value={name} onChange={(event) => setName(event.target.value)} />
      </div>

      <div className="field">
        <label className="field__label" htmlFor="org-category">
          Вид деятельности
        </label>
        <select id="org-category" value={categoryCode} onChange={(event) => setCategoryCode(event.target.value)}>
          {catalog.data.business_categories.map((item) => (
            <option key={item.code} value={item.code}>
              {item.title}
            </option>
          ))}
        </select>
      </div>

      <div className="field">
        <label className="field__label" htmlFor="org-region">
          Регион
        </label>
        <select
          id="org-region"
          value={regionCode}
          onChange={(event) => {
            const code = event.target.value;
            setRegionCode(code);
            const region = catalog.data.regions.find((item) => item.code === code);
            if (region) setTimezone(region.default_timezone);
          }}
        >
          {catalog.data.regions.map((item) => (
            <option key={item.code} value={item.code}>
              {item.title}
            </option>
          ))}
        </select>
      </div>

      <div className="field">
        <label className="field__label" htmlFor="org-timezone">
          Часовой пояс
        </label>
        <select id="org-timezone" value={timezone} onChange={(event) => setTimezone(event.target.value)}>
          {Array.from(new Set(catalog.data.regions.map((item) => item.default_timezone))).map((zone) => (
            <option key={zone} value={zone}>
              {zone}
            </option>
          ))}
        </select>
      </div>

      <section className="stack stack--tight" aria-label="Признаки организации">
        <h2 className="card__title">Что относится к вашему бизнесу</h2>
        {catalog.data.features.map((feature) => (
          <div key={feature.code} className="row row--between">
            <span className="grow">
              {feature.question}
              {feature.hint ? <span className="field__hint"> {feature.hint}</span> : null}
            </span>
            <Switch
              checked={features.includes(feature.code)}
              aria-label={feature.question}
              onChange={(event) =>
                setFeatures((prev) =>
                  event.target.checked ? [...prev, feature.code] : prev.filter((code) => code !== feature.code),
                )
              }
            />
          </div>
        ))}
      </section>

      {error ? (
        <p className="field__error" role="alert" aria-live="polite">
          {error}
        </p>
      ) : null}
    </AppShell>
  );
}
