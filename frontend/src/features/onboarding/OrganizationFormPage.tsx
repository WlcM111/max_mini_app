import { useEffect, useState } from 'react';
import { Button, Input } from '@maxhub/max-ui';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { validateOrganizationName, validateRequiredCode, validateTimezone } from '../../shared/lib/validation';
import { getCatalog } from '../organizations/api';
import { getDraft, updateDraft } from './onboardingDraft';

/** Шаг 1 онбординга: профиль бизнеса (черновик хранится локально). */
export function OrganizationFormPage() {
  const navigate = useNavigate();
  const draft = getDraft();
  const [name, setName] = useState(draft.name);
  const [categoryCode, setCategoryCode] = useState(draft.businessCategoryCode);
  const [regionCode, setRegionCode] = useState(draft.regionCode);
  const [timezone, setTimezone] = useState(draft.timezone);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });

  useEffect(() => {
    if (!catalog.data) return;
    if (categoryCode === '' && catalog.data.business_categories[0]) {
      setCategoryCode(catalog.data.business_categories[0].code);
    }
    if (regionCode === '' && catalog.data.regions[0]) {
      setRegionCode(catalog.data.regions[0].code);
      setTimezone(catalog.data.regions[0].default_timezone);
    }
  }, [catalog.data, categoryCode, regionCode]);

  if (catalog.isLoading) {
    return (
      <AppShell title="Ваша организация">
        <LoadingView rows={4} />
      </AppShell>
    );
  }
  if (catalog.isError || !catalog.data) {
    return (
      <AppShell title="Ваша организация">
        <ErrorView error={catalog.error} onRetry={() => void catalog.refetch()} />
      </AppShell>
    );
  }

  const next = () => {
    const found: Record<string, string> = {};
    const nameError = validateOrganizationName(name);
    if (nameError) found.name = nameError;
    const categoryError = validateRequiredCode(categoryCode);
    if (categoryError) found.category = categoryError;
    const regionError = validateRequiredCode(regionCode);
    if (regionError) found.region = regionError;
    const timezoneError = validateTimezone(timezone);
    if (timezoneError) found.timezone = timezoneError;
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    updateDraft({ name: name.trim(), businessCategoryCode: categoryCode, regionCode, timezone });
    navigate('/onboarding/features');
  };

  const timezones = Array.from(new Set(catalog.data.regions.map((region) => region.default_timezone)));

  return (
    <AppShell
      title="Ваша организация"
      subtitle="Шаг 1 из 4"
      actions={
        <Button size="large" stretched onClick={next}>
          Далее
        </Button>
      }
    >
      <div className="field">
        <label className="field__label" htmlFor="onboarding-name">
          Название
        </label>
        <Input
          id="onboarding-name"
          value={name}
          placeholder="Кафе «Пример»"
          onChange={(event) => setName(event.target.value)}
        />
        {errors.name ? (
          <span className="field__error" role="alert">
            {errors.name}
          </span>
        ) : null}
      </div>

      <div className="field">
        <label className="field__label" htmlFor="onboarding-category">
          Вид деятельности
        </label>
        <select id="onboarding-category" value={categoryCode} onChange={(event) => setCategoryCode(event.target.value)}>
          {catalog.data.business_categories.map((item) => (
            <option key={item.code} value={item.code}>
              {item.title}
            </option>
          ))}
        </select>
      </div>

      <div className="field">
        <label className="field__label" htmlFor="onboarding-region">
          Регион
        </label>
        <select
          id="onboarding-region"
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
        <label className="field__label" htmlFor="onboarding-timezone">
          Часовой пояс
        </label>
        <select id="onboarding-timezone" value={timezone} onChange={(event) => setTimezone(event.target.value)}>
          {timezones.map((zone) => (
            <option key={zone} value={zone}>
              {zone}
            </option>
          ))}
        </select>
        <span className="field__hint">В этом поясе приходят напоминания</span>
      </div>
    </AppShell>
  );
}
