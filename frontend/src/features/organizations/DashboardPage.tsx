import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import type { DocumentStats } from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { AppShell } from '../../shared/ui/AppShell';
import { Banner } from '../../shared/ui/Banner';
import { Button } from '../../shared/ui/Button';
import { Fab } from '../../shared/ui/Fab';
import { Icon } from '../../shared/ui/Icon';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { toast } from '../../shared/ui/Toast';
import { formatDate } from '../../shared/lib/dates';
import { plural } from '../../shared/lib/plural';
import { openMaxDeepLink } from '../../platform/max/links';
import { roleAllows, useRole, useSession } from '../../session/useSession';
import { setLastOrganization } from '../../session/sessionStore';
import { listDocuments } from '../documents/api';
import { DocumentListItem } from '../documents/DocumentListItem';
import { useMemberNames } from '../members/useMemberNames';
import { getCatalog, getOrganization } from './api';
import { OrganizationSwitcher } from './OrganizationSwitcher';

const DOCS: [string, string, string] = ['документ', 'документа', 'документов'];
const isOne = (count: number) => plural(count, ['1', '2', '5']) === '1';

/** Одна фраза о главном: что просрочено и что скоро истечёт. */
function healthLead(stats: DocumentStats): string {
  if (stats.expired > 0) {
    const head = `${stats.expired} ${plural(stats.expired, DOCS)} ${isOne(stats.expired) ? 'просрочен' : 'просрочены'}`;
    if (stats.expiring === 0) return head;
    return `${head}, ещё ${stats.expiring} скоро ${isOne(stats.expiring) ? 'истечёт' : 'истекут'}`;
  }
  if (stats.expiring > 0) {
    return `${stats.expiring} ${plural(stats.expiring, DOCS)} скоро ${isOne(stats.expiring) ? 'истечёт' : 'истекут'}`;
  }
  return 'Все сроки в порядке';
}

const TILES = [
  { code: 'expired', label: 'Просрочено' },
  { code: 'expiring', label: 'Скоро истекает' },
  { code: 'valid', label: 'В порядке' },
  { code: 'no_expiry', label: 'Бессрочные' },
] as const;

/** Панель состояния: фраза-итог, сегментная шкала и плитки-фильтры. */
function HealthPanel({ stats, onSelect }: { stats: DocumentStats; onSelect: (status: string) => void }) {
  const next = stats.next_valid_until;
  const summary = `Всего ${stats.total} ${plural(stats.total, DOCS)}${next ? `. Ближайший срок — ${formatDate(next)}` : ''}`;
  const segments = TILES.filter((tile) => stats[tile.code] > 0);
  return (
    <section className="health" aria-labelledby="health-title">
      <div>
        <h2 className="health__lead" id="health-title">
          {healthLead(stats)}
        </h2>
        <p className="health__sub">{summary}</p>
      </div>
      <div className="health__bar" role="img" aria-label={TILES.map((tile) => `${tile.label}: ${stats[tile.code]}`).join(', ')}>
        {segments.map((tile) => (
          <span key={tile.code} className={`health__seg health__seg--${tile.code}`} style={{ flexGrow: stats[tile.code] }} />
        ))}
      </div>
      <div className="stat-grid">
        {TILES.map((tile) => (
          <button
            key={tile.code}
            type="button"
            className={`stat stat--${tile.code}`}
            data-zero={stats[tile.code] === 0 ? 'true' : 'false'}
            onClick={() => onSelect(tile.code)}
          >
            <span className="stat__value">{stats[tile.code]}</span>
            <span className="stat__label">{tile.label}</span>
          </button>
        ))}
      </div>
    </section>
  );
}

/** Главная организации: состояние сроков, ближайшие документы, быстрые действия. */
export function DashboardPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const { me } = useSession();
  const role = useRole(orgId);
  const canEdit = roleAllows(role, 'editor');
  const [switcherOpen, setSwitcherOpen] = useState(false);
  const [mine, setMine] = useState(false);

  useEffect(() => {
    if (orgId) setLastOrganization(orgId);
  }, [orgId]);

  const organization = useQuery({
    queryKey: queryKeys.organization(orgId),
    queryFn: () => getOrganization(orgId),
    enabled: orgId !== '',
  });
  const upcoming = useQuery({
    queryKey: queryKeys.documents(orgId, mine ? { limit: 5, responsible: 'me' } : { limit: 5 }),
    queryFn: () => listDocuments(orgId, mine ? { limit: 5, responsible: 'me' } : { limit: 5 }),
    enabled: orgId !== '',
  });
  const catalog = useQuery({ queryKey: queryKeys.catalog(), queryFn: getCatalog, staleTime: Infinity });
  const names = useMemberNames(orgId, (upcoming.data?.items ?? []).some((item) => item.responsible_account_id));

  const membershipName = me.memberships.find((item) => item.organization_id === orgId)?.organization_name ?? 'Организация';
  const channel = me.reminders_channel;
  const openBot = () => {
    if (!channel.bot_chat_url) return;
    void openMaxDeepLink(channel.bot_chat_url).catch(() => toast('Ссылка на чат скопирована — откройте её в MAX'));
  };
  const channelBanner =
    channel.state === 'active' ? null : (
      <Banner tone="warning" icon="bell-off" title="Напоминания не приходят.">
        <span>
          {channel.state === 'stopped' || channel.state === 'unreachable'
            ? 'Откройте чат с ботом и нажмите «Старт».'
            : channel.state === 'muted'
              ? 'Уведомления бота отключены в настройках чата MAX.'
              : 'Состояние канала уточняется.'}
        </span>
        {channel.bot_chat_url ? (
          <Button size="s" variant="neutral" icon="chat" onClick={openBot}>
            Открыть чат с ботом
          </Button>
        ) : null}
      </Banner>
    );

  const fab = canEdit ? <Fab icon="plus" label="Добавить документ" onClick={() => navigate(`/o/${orgId}/documents/new`)} /> : null;
  const goToList = (status?: string) =>
    navigate(status ? `/o/${orgId}/documents?status=${status}` : `/o/${orgId}/documents`, { state: { fromHome: true } });
  const pickTypical = () => navigate('/onboarding/suggestions', { state: { from: 'dashboard' } });

  if (organization.isLoading) {
    return (
      <AppShell key={orgId} title="Загрузка" titleSkeleton wide banners={channelBanner}>
        <LoadingView rows={4} variant="card" />
      </AppShell>
    );
  }

  if (organization.isError || !organization.data) {
    return (
      <AppShell key={orgId} title={membershipName} wide>
        <ErrorView error={organization.error} onRetry={() => void organization.refetch()} />
      </AppShell>
    );
  }

  const org = organization.data;
  const stats = org.stats;
  const category = catalog.data?.business_categories.find((item) => item.code === org.business_category_code)?.title;
  const region = catalog.data?.regions.find((item) => item.code === org.region_code)?.title;
  const subtitle = [category, region].filter(Boolean).join(' · ');

  return (
    <AppShell
      key={orgId}
      title={org.name}
      subtitle={subtitle || undefined}
      onTitleClick={() => setSwitcherOpen(true)}
      banners={channelBanner}
      fab={fab}
      wide
    >
      <div className="dash">
        <div className="dash__side">
          {stats.total > 0 ? <HealthPanel stats={stats} onSelect={goToList} /> : null}
          {canEdit && stats.total > 0 ? (
            <button type="button" className="promo" onClick={pickTypical}>
              <span className="promo__icon" aria-hidden="true">
                <Icon name="sparkles" />
              </span>
              <span className="promo__text">
                <span className="promo__title">Подобрать типовые документы</span>
                <span className="promo__hint">Покажем, что обычно нужно вашему бизнесу</span>
              </span>
              <Icon name="chevron-right" className="promo__chev" />
            </button>
          ) : null}
        </div>
        <section className="dash__main" aria-labelledby="upcoming-title">
          <div className="section-head">
            <h2 className="section-title" id="upcoming-title">
              Ближайшие сроки
            </h2>
            <Button variant="tertiary" size="s" iconRight="chevron-right" onClick={() => goToList()}>
              Все документы
            </Button>
          </div>
          <div className="segmented mine-switch" role="group" aria-label="Чьи документы показать">
            <button type="button" className="segmented__btn" aria-pressed={!mine} onClick={() => setMine(false)}>
              Все
            </button>
            <button type="button" className="segmented__btn" aria-pressed={mine} onClick={() => setMine(true)}>
              Мои
            </button>
          </div>
          {upcoming.isLoading ? <LoadingView rows={3} /> : null}
          {upcoming.isError ? <ErrorView error={upcoming.error} onRetry={() => void upcoming.refetch()} /> : null}
          {mine && upcoming.data && upcoming.data.items.length === 0 ? (
            <EmptyView
              art="people"
              title="За вами пока нет документов"
              description="Назначьте себя ответственным в карточке документа — напоминания будут приходить лично вам."
            />
          ) : null}
          {!mine && upcoming.data && upcoming.data.items.length === 0 ? (
            <EmptyView
              title="Документов пока нет"
              description={
                canEdit
                  ? 'Подберём типовые документы по профилю организации или добавьте свой через кнопку «+».'
                  : 'Когда коллеги добавят документы, они появятся здесь.'
              }
              action={
                canEdit ? (
                  <Button variant="secondary" icon="sparkles" onClick={pickTypical}>
                    Подобрать по профилю
                  </Button>
                ) : null
              }
            />
          ) : null}
          {upcoming.data && upcoming.data.items.length > 0 ? (
            <div className="list stagger">
              {upcoming.data.items.map((item) => (
                <DocumentListItem key={item.id} item={item} responsibleName={names.get(item.responsible_account_id ?? '')} onOpen={(id) => navigate(`/d/${id}`)} />
              ))}
            </div>
          ) : null}
        </section>
      </div>
      <OrganizationSwitcher open={switcherOpen} organizationId={orgId} onClose={() => setSwitcherOpen(false)} />
    </AppShell>
  );
}
