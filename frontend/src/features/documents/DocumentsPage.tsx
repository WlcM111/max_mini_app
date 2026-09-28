import { useEffect, useState } from 'react';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { Fab } from '../../shared/ui/Fab';
import { TextField } from '../../shared/ui/Field';
import { Icon } from '../../shared/ui/Icon';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { roleAllows, useRole, useSession } from '../../session/useSession';
import { getOrganization } from '../organizations/api';
import { listDocuments } from './api';
import { DeadlineCalendar } from './DeadlineCalendar';
import { DocumentListItem } from './DocumentListItem';
import { useMemberNames } from '../members/useMemberNames';

const FILTERS = [
  { code: '', title: 'Все' },
  { code: 'expired', title: 'Просрочено' },
  { code: 'expiring', title: 'Скоро' },
  { code: 'valid', title: 'В порядке' },
  { code: 'no_expiry', title: 'Бессрочные' },
] as const;

type StatsKey = 'total' | 'expired' | 'expiring' | 'valid' | 'no_expiry';
const STATS_KEY: Record<string, StatsKey> = { '': 'total', expired: 'expired', expiring: 'expiring', valid: 'valid', no_expiry: 'no_expiry' };

/** Реестр документов: поиск, фильтр по статусу со счётчиками и подгрузка по курсору. */
export function DocumentsPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const { me } = useSession();
  const role = useRole(orgId);
  const canEdit = roleAllows(role, 'editor');
  const [searchParams, setSearchParams] = useSearchParams();
  const status = searchParams.get('status') ?? '';
  const view = searchParams.get('view') === 'calendar' ? 'calendar' : 'list';
  const mine = searchParams.get('mine') === '1';
  const [searchInput, setSearchInput] = useState(searchParams.get('q') ?? '');
  const [query, setQuery] = useState(searchParams.get('q') ?? '');

  useEffect(() => {
    const timer = setTimeout(() => setQuery(searchInput.trim()), 350);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const organization = useQuery({
    queryKey: queryKeys.organization(orgId),
    queryFn: () => getOrganization(orgId),
    enabled: orgId !== '',
  });

  const documents = useInfiniteQuery({
    queryKey: queryKeys.documents(orgId, { status, q: query, ...(mine ? { responsible: 'me' } : {}) }),
    initialPageParam: '',
    queryFn: ({ pageParam }) =>
      listDocuments(orgId, {
        ...(status ? { status } : {}),
        ...(query ? { q: query } : {}),
        ...(pageParam ? { cursor: pageParam as string } : {}),
        ...(mine ? { responsible: 'me' } : {}),
        limit: 20,
      }),
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
    enabled: orgId !== '',
  });

  const items = documents.data?.pages.flatMap((page) => page.items) ?? [];
  const names = useMemberNames(orgId, items.some((item) => item.responsible_account_id));
  const stats = organization.data?.stats;
  const organizationName = me.memberships.find((item) => item.organization_id === orgId)?.organization_name;

  const setStatus = (code: string) => {
    const next = new URLSearchParams(searchParams);
    if (code) next.set('status', code);
    else next.delete('status');
    setSearchParams(next, { replace: true, state: location.state });
  };

  const setView = (next: 'list' | 'calendar') => {
    const params = new URLSearchParams(searchParams);
    if (next === 'calendar') params.set('view', 'calendar');
    else params.delete('view');
    setSearchParams(params, { replace: true, state: location.state });
  };

  const viewSwitch = (
    <div className="segmented view-switch" role="group" aria-label="Вид реестра">
      <button type="button" className="segmented__btn" aria-pressed={view === 'list'} onClick={() => setView('list')}>
        <Icon name="list" size={18} />
        Список
      </button>
      <button type="button" className="segmented__btn" aria-pressed={view === 'calendar'} onClick={() => setView('calendar')}>
        <Icon name="calendar" size={18} />
        Календарь
      </button>
    </div>
  );

  if (view === 'calendar') {
    return (
      <AppShell
        title="Документы"
        subtitle={organizationName}
        headerExtra={viewSwitch}
        wide
        fab={canEdit ? <Fab icon="plus" label="Добавить документ" onClick={() => navigate(`/o/${orgId}/documents/new`)} /> : null}
      >
        <DeadlineCalendar organizationId={orgId} timezone={organization.data?.timezone ?? 'Europe/Moscow'} onOpen={(id) => navigate(`/d/${id}`)} />
      </AppShell>
    );
  }

  return (
    <AppShell
      title="Документы"
      subtitle={organizationName}
      headerExtra={viewSwitch}
      wide
      fab={canEdit ? <Fab icon="plus" label="Добавить документ" onClick={() => navigate(`/o/${orgId}/documents/new`)} /> : null}
    >
      <div className="tools">
        <TextField
          id="documents-search"
          label="Поиск по названию"
          labelHidden
          type="search"
          inputMode="search"
          icon="search"
          placeholder="Найти документ"
          value={searchInput}
          onChange={setSearchInput}
          clearable
          raised
        />
        <div className="chips chips--scroll" role="group" aria-label="Фильтр по статусу">
          {FILTERS.map((filter) => {
            const key = STATS_KEY[filter.code];
            const count = stats && key ? stats[key] : undefined;
            return (
              <button
                key={filter.code || 'all'}
                type="button"
                className={`chip chip--${filter.code || 'all'}`}
                aria-pressed={status === filter.code}
                onClick={() => setStatus(filter.code)}
              >
                {filter.code ? <span className="chip__dot" aria-hidden="true" /> : null}
                {filter.title}
                {count !== undefined ? (
                  <span className="chip__count" aria-hidden="true">
                    {count}
                  </span>
                ) : null}
              </button>
            );
          })}
          <button
            type="button"
            className="chip chip--mine"
            aria-pressed={mine}
            onClick={() => {
              const next = new URLSearchParams(searchParams);
              if (mine) next.delete('mine');
              else next.set('mine', '1');
              setSearchParams(next, { replace: true, state: location.state });
            }}
          >
            <Icon name="user" size={16} />
            Мои
          </button>
        </div>
      </div>

      {documents.isLoading ? <LoadingView rows={5} /> : null}
      {documents.isError ? <ErrorView error={documents.error} onRetry={() => void documents.refetch()} /> : null}
      {documents.isSuccess && items.length === 0 ? (
        mine && !query && !status ? (
          <EmptyView art="people" title="У вас пока нет своих документов" description="Назначьте себя ответственным в карточке документа — напоминания будут приходить лично вам." />
        ) : query ? (
          <EmptyView
            art="search"
            title="Ничего не найдено"
            description="Проверьте запрос или поищите по другому слову из названия."
            action={
              <Button variant="secondary" onClick={() => setSearchInput('')}>
                Очистить поиск
              </Button>
            }
          />
        ) : status ? (
          <EmptyView
            art="none"
            title="Нет документов с таким статусом"
            action={
              <Button variant="secondary" onClick={() => setStatus('')}>
                Показать все
              </Button>
            }
          />
        ) : (
          <EmptyView
            title="Документов пока нет"
            description={canEdit ? 'Добавьте первый документ или загрузите сразу всю таблицу из Excel.' : undefined}
            action={
              canEdit ? (
                <Button variant="secondary" icon="upload" onClick={() => navigate(`/o/${orgId}/documents/import`)}>
                  Импорт из Excel
                </Button>
              ) : null
            }
          />
        )
      ) : null}
      {items.length > 0 ? (
        <div className="list list--grid stagger">
          {items.map((item) => (
            <DocumentListItem
              key={item.id}
              item={item}
              responsibleName={names.get(item.responsible_account_id ?? '')}
              onOpen={(id) => navigate(`/d/${id}`)}
            />
          ))}
        </div>
      ) : null}
      {documents.hasNextPage ? (
        <Button variant="neutral" stretched loading={documents.isFetchingNextPage} onClick={() => void documents.fetchNextPage()}>
          Показать ещё
        </Button>
      ) : null}
    </AppShell>
  );
}
