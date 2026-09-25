import { useEffect, useState } from 'react';
import { Button, Input } from '@maxhub/max-ui';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useNavigate, useParams, useSearchParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { roleAllows, useRole } from '../../session/useSession';
import { listDocuments } from './api';
import { DocumentListItem } from './DocumentListItem';

const FILTERS = [
  { code: '', title: 'Все' },
  { code: 'expired', title: 'Просрочено' },
  { code: 'expiring', title: 'Скоро' },
  { code: 'valid', title: 'В порядке' },
  { code: 'no_expiry', title: 'Бессрочные' },
] as const;

/** Реестр документов: фильтр по статусу, поиск и подгрузка по курсору. */
export function DocumentsPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const role = useRole(orgId);
  const [searchParams, setSearchParams] = useSearchParams();
  const status = searchParams.get('status') ?? '';
  const [searchInput, setSearchInput] = useState(searchParams.get('q') ?? '');
  const [query, setQuery] = useState(searchParams.get('q') ?? '');

  useEffect(() => {
    const timer = setTimeout(() => setQuery(searchInput.trim()), 350);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const documents = useInfiniteQuery({
    queryKey: queryKeys.documents(orgId, { status, q: query }),
    initialPageParam: '',
    queryFn: ({ pageParam }) =>
      listDocuments(orgId, {
        ...(status ? { status } : {}),
        ...(query ? { q: query } : {}),
        ...(pageParam ? { cursor: pageParam as string } : {}),
        limit: 20,
      }),
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
    enabled: orgId !== '',
  });

  const items = documents.data?.pages.flatMap((page) => page.items) ?? [];

  return (
    <AppShell
      title="Документы"
      actions={
        roleAllows(role, 'editor') ? (
          <Button size="large" stretched onClick={() => navigate(`/o/${orgId}/documents/new`)}>
            Добавить документ
          </Button>
        ) : null
      }
    >
      <div className="chips nowrap-scroll" role="group" aria-label="Фильтр по статусу">
        {FILTERS.map((filter) => (
          <button
            key={filter.code || 'all'}
            type="button"
            className="chip"
            aria-pressed={status === filter.code}
            onClick={() => {
              const next = new URLSearchParams(searchParams);
              if (filter.code) next.set('status', filter.code);
              else next.delete('status');
              setSearchParams(next, { replace: true });
            }}
          >
            {filter.title}
          </button>
        ))}
      </div>

      <div className="field">
        <label className="field__label" htmlFor="documents-search">
          Поиск по названию
        </label>
        <Input
          id="documents-search"
          value={searchInput}
          placeholder="Например, лицензия"
          onChange={(event) => setSearchInput(event.target.value)}
        />
      </div>

      {documents.isLoading ? <LoadingView rows={5} /> : null}
      {documents.isError ? <ErrorView error={documents.error} onRetry={() => void documents.refetch()} /> : null}

      {documents.isSuccess && items.length === 0 ? (
        <EmptyView
          title={query ? 'Ничего не найдено' : status ? 'Нет документов с таким статусом' : 'Документов пока нет'}
          description={query ? 'Измените запрос или очистите поиск.' : undefined}
        />
      ) : null}

      <div className="list">
        {items.map((item) => (
          <DocumentListItem key={item.id} item={item} onOpen={(id) => navigate(`/d/${id}`)} />
        ))}
      </div>

      {documents.hasNextPage ? (
        <Button
          size="medium"
          stretched
          variant="secondary"
          loading={documents.isFetchingNextPage}
          onClick={() => void documents.fetchNextPage()}
        >
          Показать ещё
        </Button>
      ) : null}
    </AppShell>
  );
}
