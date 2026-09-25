import { useEffect } from 'react';
import { Button } from '@maxhub/max-ui';
import { useQuery } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { StatusBadge } from '../../shared/ui/StatusBadge';
import { formatDate } from '../../shared/lib/dates';
import { roleAllows, useRole, useSession } from '../../session/useSession';
import { setLastOrganization } from '../../session/sessionStore';
import { openMaxDeepLink } from '../../platform/max/links';
import { listDocuments } from '../documents/api';
import { getOrganization } from './api';
import { OrganizationSwitcher } from './OrganizationSwitcher';

/** Дашборд организации: счётчики статусов и ближайшие сроки (FR-06). */
export function DashboardPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const { me } = useSession();
  const role = useRole(orgId);

  useEffect(() => {
    if (orgId) setLastOrganization(orgId);
  }, [orgId]);

  const organization = useQuery({
    queryKey: queryKeys.organization(orgId),
    queryFn: () => getOrganization(orgId),
    enabled: orgId !== '',
  });

  const upcoming = useQuery({
    queryKey: queryKeys.documents(orgId, { limit: 5 }),
    queryFn: () => listDocuments(orgId, { limit: 5 }),
    enabled: orgId !== '',
  });

  const channel = me.reminders_channel;
  const channelBanner =
    channel.state === 'active' ? null : (
      <div className="banner banner--warning">
        <strong>Напоминания не приходят.</strong>{' '}
        {channel.state === 'stopped' || channel.state === 'unreachable'
          ? 'Откройте чат с ботом и нажмите «Старт».'
          : channel.state === 'muted'
            ? 'Уведомления бота отключены в настройках чата MAX.'
            : 'Состояние канала уточняется.'}
        {channel.bot_chat_url ? (
          <>
            {' '}
            <Button
              size="xsmall"
              variant="ghost"
              onClick={() => {
                if (channel.bot_chat_url) void openMaxDeepLink(channel.bot_chat_url);
              }}
            >
              Открыть чат с ботом
            </Button>
          </>
        ) : null}
      </div>
    );

  if (organization.isLoading) {
    return (
      <AppShell title="Загрузка…">
        <LoadingView rows={4} />
      </AppShell>
    );
  }
  if (organization.isError || !organization.data) {
    return (
      <AppShell title="Организация">
        <ErrorView error={organization.error} onRetry={() => void organization.refetch()} />
      </AppShell>
    );
  }

  const org = organization.data;
  const stats = org.stats;
  const goToList = (status?: string) =>
    navigate(status ? `/o/${orgId}/documents?status=${status}` : `/o/${orgId}/documents`);

  return (
    <AppShell
      title={org.name}
      subtitle={`Документов: ${stats.total}`}
      banners={channelBanner}
      actions={
        <>
          {roleAllows(role, 'editor') ? (
            <Button size="large" stretched onClick={() => navigate(`/o/${orgId}/documents/new`)}>
              Добавить документ
            </Button>
          ) : null}
          <Button size="medium" stretched variant="secondary" onClick={() => goToList()}>
            Все документы
          </Button>
        </>
      }
    >
      <OrganizationSwitcher organizationId={orgId} />

      <div className="stat-grid">
        <button type="button" className="stat" onClick={() => goToList('expired')}>
          <span className="stat__value">{stats.expired}</span>
          <span className="stat__label">Просрочено</span>
        </button>
        <button type="button" className="stat" onClick={() => goToList('expiring')}>
          <span className="stat__value">{stats.expiring}</span>
          <span className="stat__label">Скоро истекает</span>
        </button>
        <button type="button" className="stat" onClick={() => goToList('valid')}>
          <span className="stat__value">{stats.valid}</span>
          <span className="stat__label">В порядке</span>
        </button>
        <button type="button" className="stat" onClick={() => goToList('no_expiry')}>
          <span className="stat__value">{stats.no_expiry}</span>
          <span className="stat__label">Бессрочные</span>
        </button>
      </div>

      <section className="stack stack--tight" aria-label="Ближайшие сроки">
        <h2 className="card__title">Ближайшие сроки</h2>
        {upcoming.isLoading ? <LoadingView rows={3} /> : null}
        {upcoming.isError ? <ErrorView error={upcoming.error} onRetry={() => void upcoming.refetch()} /> : null}
        {upcoming.data && upcoming.data.items.length === 0 ? (
          <EmptyView
            title="Документов пока нет"
            description="Добавьте первый документ или подберите типовые по профилю организации."
            action={
              roleAllows(role, 'editor') ? (
                <div className="row row--wrap">
                  <Button size="medium" onClick={() => navigate(`/o/${orgId}/documents/new`)}>
                    Добавить документ
                  </Button>
                  <Button size="medium" variant="secondary" onClick={() => navigate('/onboarding/suggestions')}>
                    Подобрать по профилю
                  </Button>
                </div>
              ) : null
            }
          />
        ) : null}
        <div className="list">
          {upcoming.data?.items.map((item) => (
            <button key={item.id} type="button" className="list-item" onClick={() => navigate(`/d/${item.id}`)}>
              <span className="grow">
                <span className="list-item__title truncate">{item.title}</span>
                <span className="list-item__meta">
                  {item.valid_until ? `до ${formatDate(item.valid_until)}` : 'бессрочный'}
                  {item.reminders_state === 'pending' ? ' · напоминания пересчитываются' : ''}
                  {item.reminders_state === 'unavailable' ? ' · план временно недоступен' : ''}
                </span>
              </span>
              <StatusBadge status={item.status} daysLeft={item.days_left} withDays={false} />
            </button>
          ))}
        </div>
      </section>

      <div className="row row--wrap">
        <Button size="medium" variant="secondary" onClick={() => navigate(`/o/${orgId}/members`)}>
          Участники
        </Button>
        <Button size="medium" variant="secondary" onClick={() => navigate(`/o/${orgId}/settings`)}>
          Настройки
        </Button>
        <Button size="medium" variant="ghost" onClick={() => navigate('/account')}>
          Аккаунт
        </Button>
      </div>
    </AppShell>
  );
}
