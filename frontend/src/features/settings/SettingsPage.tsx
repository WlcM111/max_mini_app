import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { SelectField } from '../../shared/ui/Field';
import { NavRow } from '../../shared/ui/Rows';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { ToggleRow } from '../../shared/ui/Switch';
import { toast } from '../../shared/ui/Toast';
import { cx } from '../../shared/lib/cx';
import { timezoneLabel } from '../../shared/lib/format';
import { notifyTimeOptions, validateNotifyTime } from '../../shared/lib/validation';
import { openMaxDeepLink } from '../../platform/max/links';
import { clearLastOrganization } from '../../session/sessionStore';
import { roleAllows, useRole, useSession } from '../../session/useSession';
import { CalendarExportButton } from '../export/CalendarExportButton';
import { RegistryExportButton } from '../export/RegistryExportButton';
import { removeMember } from '../members/api';
import { deleteOrganization, getOrganization } from '../organizations/api';
import { getNotificationSettings, putNotificationSettings } from './api';

const CHANNEL_TEXT: Record<string, string> = {
  active: 'Напоминания приходят в чат с ботом',
  muted: 'Уведомления бота отключены в настройках чата MAX',
  stopped: 'Бот остановлен — откройте чат и нажмите «Старт»',
  unreachable: 'Чат с ботом недоступен — откройте его в MAX',
  unknown: 'Состояние канала уточняется',
  unavailable: 'Состояние канала временно недоступно',
};

/** Настройки: напоминания, канал MAX, экспорт, профиль, выход и удаление. */
export function SettingsPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { me, refreshMe } = useSession();
  const role = useRole(orgId);
  const isOwner = roleAllows(role, 'owner');
  const canEdit = roleAllows(role, 'editor');
  const [confirm, setConfirm] = useState<'delete-org' | 'leave' | null>(null);
  const [enabled, setEnabled] = useState(true);
  const [localTime, setLocalTime] = useState('09:00');

  const organization = useQuery({
    queryKey: queryKeys.organization(orgId),
    queryFn: () => getOrganization(orgId),
    enabled: orgId !== '',
  });
  const settings = useQuery({
    queryKey: queryKeys.notify(orgId),
    queryFn: () => getNotificationSettings(orgId),
    enabled: orgId !== '',
    staleTime: 60_000,
  });

  useEffect(() => {
    if (settings.data) {
      setEnabled(settings.data.enabled);
      setLocalTime(settings.data.local_time);
    }
  }, [settings.data]);

  const save = useMutation({
    mutationFn: (next: { enabled: boolean; local_time: string }) => putNotificationSettings(orgId, next),
    onSuccess: async (data) => {
      queryClient.setQueryData(queryKeys.notify(orgId), data);
      await refreshMe();
      toast('Настройки сохранены', 'success');
    },
    onError: (error) => toast(messageForError(error), 'error'),
  });

  const removeOrganization = useMutation({
    mutationFn: () => deleteOrganization(orgId),
    onSuccess: async () => {
      clearLastOrganization();
      await refreshMe();
      await queryClient.invalidateQueries();
      navigate('/', { replace: true });
    },
    onError: (error) => {
      setConfirm(null);
      toast(messageForError(error), 'error');
    },
  });

  const leave = useMutation({
    mutationFn: () => removeMember(orgId, me.account.id),
    onSuccess: async () => {
      clearLastOrganization();
      await refreshMe();
      navigate('/', { replace: true });
    },
    onError: (error) => {
      setConfirm(null);
      toast(messageForError(error), 'error');
    },
  });

  const applySettings = (next: { enabled: boolean; local_time: string }) => {
    const invalid = validateNotifyTime(next.local_time);
    if (invalid) {
      toast(invalid, 'error');
      return;
    }
    save.mutate(next);
  };

  if (organization.isLoading || settings.isLoading) {
    return (
      <AppShell title="Настройки">
        <LoadingView rows={4} variant="form" />
      </AppShell>
    );
  }

  if (organization.isError || !organization.data) {
    return (
      <AppShell title="Настройки">
        <ErrorView error={organization.error} onRetry={() => void organization.refetch()} />
      </AppShell>
    );
  }

  const channel = me.reminders_channel;
  const channelWarn = channel.state !== 'active';
  const fullName = [me.account.first_name, me.account.last_name].filter(Boolean).join(' ');
  const openBot = () => {
    if (channel.bot_chat_url) void openMaxDeepLink(channel.bot_chat_url).catch(() => toast('Ссылка на чат скопирована — откройте её в MAX'));
  };

  return (
    <AppShell title="Настройки" subtitle={organization.data.name} layout="columns">
      <section className="group" aria-labelledby="notify-title">
        <h2 className="group__title" id="notify-title">
          Мои напоминания
        </h2>
        <div className="group__card">
          <ToggleRow
            title="Присылать напоминания"
            hint="Сообщения приходят в чат с ботом, по понедельникам — сводка на неделю"
            checked={enabled}
            disabled={save.isPending}
            onChange={(checked) => {
              setEnabled(checked);
              applySettings({ enabled: checked, local_time: localTime });
            }}
          />
          <div className="group__field">
            <SelectField
              id="notify-time"
              label="Время напоминаний"
              hint={`Часовой пояс: ${timezoneLabel(organization.data.timezone)}`}
              value={localTime}
              disabled={!enabled || save.isPending}
              onChange={(value) => {
                setLocalTime(value);
                applySettings({ enabled, local_time: value });
              }}
              options={notifyTimeOptions().map((option) => ({ value: option, label: option }))}
            />
          </div>
          <div className={cx('channel', channelWarn && 'channel--warn')}>
            <span className="channel__dot" aria-hidden="true" />
            <span>{CHANNEL_TEXT[channel.state] ?? CHANNEL_TEXT.unknown}</span>
          </div>
          {channelWarn && channel.bot_chat_url ? (
            <div className="group__field">
              <Button variant="neutral" icon="chat" onClick={openBot}>
                Открыть чат с ботом
              </Button>
            </div>
          ) : null}
        </div>
      </section>

      <section className="group" aria-labelledby="calendar-title">
        <h2 className="group__title" id="calendar-title">
          Календарь
        </h2>
        <div className="group__card group__card--pad">
          <p className="group__text">Сроки всех документов одним файлом .ics — откройте его в календаре телефона или компьютера.</p>
          <CalendarExportButton organizationId={orgId} />
        </div>
      </section>

      <section className="group" aria-labelledby="data-title">
        <h2 className="group__title" id="data-title">
          Данные
        </h2>
        <div className="group__card">
          {canEdit ? (
            <NavRow
              icon="upload"
              title="Импорт из Excel"
              hint="Загрузить сразу много документов из таблицы"
              onClick={() => navigate(`/o/${orgId}/documents/import`)}
            />
          ) : null}
          <div className="group__field">
            <p className="group__text">Реестр со сроками и статусами — для бухгалтерии и проверок.</p>
            <RegistryExportButton organizationId={orgId} />
          </div>
        </div>
      </section>

      <section className="group" aria-labelledby="org-title">
        <h2 className="group__title" id="org-title">
          Организация и аккаунт
        </h2>
        <div className="group__card">
          {canEdit ? (
            <NavRow
              icon="building"
              title="Профиль организации"
              hint="Название, вид деятельности, регион"
              onClick={() => navigate(`/o/${orgId}/settings/profile`)}
            />
          ) : null}
          <NavRow icon="user" title="Аккаунт" hint={fullName} onClick={() => navigate('/account')} />
        </div>
      </section>

      <section className="group" aria-label="Опасные действия">
        <div className="group__card">
          <NavRow
            icon={isOwner ? 'trash' : 'logout'}
            title={isOwner ? 'Удалить организацию' : 'Выйти из организации'}
            danger
            chevron={false}
            onClick={() => setConfirm(isOwner ? 'delete-org' : 'leave')}
          />
        </div>
        <p className="group__foot">
          {isOwner ? 'Удалятся все документы, напоминания и приглашения организации.' : 'Доступ к документам организации пропадёт.'}
        </p>
      </section>

      <ConfirmDialog
        open={confirm !== null}
        title={confirm === 'delete-org' ? 'Удалить организацию?' : 'Выйти из организации?'}
        description={
          confirm === 'delete-org'
            ? 'Все документы, напоминания и приглашения будут удалены. Действие нельзя отменить.'
            : 'Вы потеряете доступ к документам этой организации.'
        }
        confirmLabel={confirm === 'delete-org' ? 'Удалить' : 'Выйти'}
        destructive
        pending={removeOrganization.isPending || leave.isPending}
        onConfirm={() => (confirm === 'delete-org' ? removeOrganization.mutate() : leave.mutate())}
        onCancel={() => setConfirm(null)}
      />
    </AppShell>
  );
}
