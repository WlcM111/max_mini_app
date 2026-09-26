import { useEffect, useState } from 'react';
import { Button, Input, Textarea } from '@maxhub/max-ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { validateOrganizationName, validateRequiredCode, validateTimezone } from '../../shared/lib/validation';
import { getCatalog, matchProfile } from '../organizations/api';
import { useSession } from '../../session/useSession';
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
  const { me } = useSession();

  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });

  // Подбор профиля по свободному описанию (FR-22): ассистент возвращает только
  // коды справочника, пользователь видит результат и может его изменить.
  const match = useMutation({
    mutationFn: () => matchProfile(description),
    onSuccess: (result) => {
      if (result.business_category_code) setCategoryCode(result.business_category_code);
      updateDraft({ featureCodes: [...result.feature_codes] });
      const category = catalog.data?.business_categories.find(
        (item) => item.code === result.business_category_code,
      );
      setNotice(
        result.business_category_code || result.feature_codes.length > 0
          ? `Подобрано: ${category?.title ?? 'вид деятельности не определён'}` +
              `, признаков: ${result.feature_codes.length}. Проверьте и продолжите.`
          : 'По описанию ничего не подобрано — заполните поля вручную',
      );
    },
    onError: (error) => setNotice(messageForError(error)),
  });

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
      {me.assistant_enabled ? (
        <section className="card stack stack--tight" aria-label="Подбор по описанию">
          <h2 className="card__title">Опишите бизнес своими словами</h2>
          <p className="card__text">
            По описанию подберём вид деятельности и признаки — останется проверить и продолжить.
          </p>
          <Textarea
            aria-label="Описание бизнеса"
            value={description}
            placeholder="Кофейня на 30 мест, есть летняя веранда, продаём пиво"
            onChange={(event) => setDescription(event.target.value)}
          />
          <Button
            size="medium"
            variant="secondary"
            loading={match.isPending}
            disabled={match.isPending || description.trim() === ''}
            onClick={() => match.mutate()}
          >
            Подобрать по описанию
          </Button>
          {notice ? (
            <p className="muted" aria-live="polite">
              {notice}
            </p>
          ) : null}
          <span className="field__hint">Текст обрабатывается сервисом GigaChat; данные организации не передаются.</span>
        </section>
      ) : null}

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
