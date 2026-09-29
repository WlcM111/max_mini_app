import { useEffect, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { assistantErrorMessage } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { SelectField, TextAreaField, TextField } from '../../shared/ui/Field';
import { Icon } from '../../shared/ui/Icon';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { Steps } from '../../shared/ui/Steps';
import { timezoneLabel } from '../../shared/lib/format';
import { validateOrganizationName, validateRequiredCode, validateTimezone } from '../../shared/lib/validation';
import { useSession } from '../../session/useSession';
import { getCatalog, matchProfile } from '../organizations/api';
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
  const [description, setDescription] = useState('');
  const [notice, setNotice] = useState<string | null>(null);
  const [flashCategory, setFlashCategory] = useState(false);
  const { me } = useSession();
  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });

  // Подбор профиля по свободному описанию (FR-22): ассистент возвращает только
  // коды справочника, пользователь видит результат и может его изменить.
  const match = useMutation({
    mutationFn: () => matchProfile(description),
    onSuccess: (result) => {
      if (result.business_category_code) {
        setCategoryCode(result.business_category_code);
        setFlashCategory(true);
      }
      updateDraft({ featureCodes: [...result.feature_codes] });
      const category = catalog.data?.business_categories.find((item) => item.code === result.business_category_code);
      setNotice(
        result.business_category_code || result.feature_codes.length > 0
          ? `Подобрано: ${category?.title ?? 'вид деятельности не определён'}` +
              `, признаков: ${result.feature_codes.length}. Проверьте и продолжите.`
          : 'По описанию ничего не подобрано — заполните поля вручную',
      );
    },
    onError: (error) => setNotice(assistantErrorMessage(error, 'profile')),
  });

  useEffect(() => {
    if (!flashCategory) return undefined;
    const timer = window.setTimeout(() => setFlashCategory(false), 1500);
    return () => window.clearTimeout(timer);
  }, [flashCategory]);

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

  const steps = <Steps current={1} total={4} />;

  if (catalog.isLoading) {
    return (
      <AppShell title="Ваша организация" headerExtra={steps}>
        <LoadingView rows={4} variant="form" />
      </AppShell>
    );
  }

  if (catalog.isError || !catalog.data) {
    return (
      <AppShell title="Ваша организация" headerExtra={steps}>
        <ErrorView error={catalog.error} onRetry={() => void catalog.refetch()} />
      </AppShell>
    );
  }

  const data = catalog.data;
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

  const timezones = Array.from(new Set(data.regions.map((region) => region.default_timezone)));

  return (
    <AppShell
      layout="split"
      title="Ваша организация"
      headerExtra={steps}
      actions={
        <Button size="l" stretched iconRight="arrow-right" onClick={next}>
          Далее
        </Button>
      }
    >
      {me.assistant_enabled ? (
        <section className="assist" aria-labelledby="profile-assist-title">
          <div className="assist__head">
            <span className="assist__icon" aria-hidden="true">
              <Icon name="sparkles" />
            </span>
            <div>
              <h2 className="assist__title" id="profile-assist-title">
                Опишите бизнес своими словами
              </h2>
              <p className="assist__text">Подберём вид деятельности и особенности — останется только проверить.</p>
            </div>
          </div>
          <TextAreaField
            label="Описание бизнеса"
            labelHidden
            value={description}
            onChange={setDescription}
            rows={3}
            placeholder="Кофейня на 30 мест, летняя веранда, продаём пиво"
          />
          <Button
            variant="secondary"
            icon="sparkles"
            loading={match.isPending}
            disabled={description.trim() === ''}
            onClick={() => match.mutate()}
          >
            Подобрать по описанию
          </Button>
          {notice ? (
            <p className="assist__notice" aria-live="polite">
              {notice}
            </p>
          ) : null}
          <p className="assist__fine">Описание обрабатывается сервисом GigaChat и не сохраняется.</p>
        </section>
      ) : null}
      <section className="group" aria-labelledby="org-form-title">
        <h2 className="group__title" id="org-form-title">
          Основное
        </h2>
        <div className="form-card">
          <TextField
            id="onboarding-name"
            label="Название"
            value={name}
            error={errors.name}
            placeholder="Кафе «Пример»"
            maxLength={200}
            onChange={setName}
          />
          <SelectField
            id="onboarding-category"
            label="Вид деятельности"
            value={categoryCode}
            error={errors.category}
            flash={flashCategory}
            onChange={setCategoryCode}
            options={data.business_categories.map((item) => ({ value: item.code, label: item.title }))}
          />
          <SelectField
            id="onboarding-region"
            label="Регион"
            value={regionCode}
            error={errors.region}
            onChange={(code) => {
              setRegionCode(code);
              const region = data.regions.find((item) => item.code === code);
              if (region) setTimezone(region.default_timezone);
            }}
            options={data.regions.map((item) => ({ value: item.code, label: item.title }))}
          />
          <SelectField
            id="onboarding-timezone"
            label="Часовой пояс"
            hint="В этом поясе приходят напоминания"
            value={timezone}
            error={errors.timezone}
            onChange={setTimezone}
            options={timezones.map((zone) => ({ value: zone, label: timezoneLabel(zone) }))}
          />
        </div>
      </section>
    </AppShell>
  );
}
