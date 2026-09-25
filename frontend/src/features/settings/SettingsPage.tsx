import { useEffect, useState } from 'react';
import { Button, Switch } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { notifyTimeOptions, validateNotifyTime } from '../../shared/lib/validation';
import { openMaxDeepLink } from '../../platform/max/links';
import { clearLastOrganization } from '../../session/sessionStore';
import { roleAllows, useRole, useSession } from '../../session/useSession';
import { CalendarExportButton } from '../export/CalendarExportButton';
import { deleteOrganization, getOrganization } from '../organizations/api';
import { removeMember } from '../members/api';
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
  const [confirm, setConfirm] = useState<'delete-org' | 'leave' | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
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
      setNotice('Настройки сохранены');
    },
    onError: (error) => setNotice(messageForError(error)),
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
      setNotice(messageForError(error));
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
      setNotice(messageForError(error));
    },
  });

  const applySettings = (next: { enabled: boolean; local_time: string }) => {
    const invalid = validateNotifyTime(next.local_time);
    if (invalid) {
      setNotice(invalid);
      return;
    }
    setNotice(null);
    save.mutate(next);
  };

  if (organization.isLoading || settings.isLoading) {
    return (
      <AppShell title="Настройки">
        <LoadingView rows={4} />
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

  return (
    <AppShell title="Настройки" subtitle={organization.data.name}>
      <section className="card stack stack--tight" aria-label="Мои напоминания">
        <h2 className="card__title">Мои напоминания</h2>
        <div className="row row--between">
          <span>Присылать напоминания</span>
          <Switch
            checked={enabled}
            aria-label="Присылать напоминания"
            onChange={(event) => {
              setEnabled(event.target.checked);
              applySettings({ enabled: event.target.checked, local_time: localTime });
            }}
          />
        </div>
        <div className="field">
          <label className="field__label" htmlFor="notify-time">
            Время напоминаний ({organization.data.timezone})
          </label>
          <select
            id="notify-time"
            value={localTime}
            onChange={(event) => {
              setLocalTime(event.target.value);
              applySettings({ enabled, local_time: event.target.value });
            }}
          >
            {notifyTimeOptions().map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </div>
        <p className="muted">{CHANNEL_TEXT[channel.state] ?? CHANNEL_TEXT.unknown}</p>
        {channel.bot_chat_url && channel.state !== 'active' ? (
          <Button
            size="medium"
            variant="secondary"
            onClick={() => {
              if (channel.bot_chat_url) void openMaxDeepLink(channel.bot_chat_url);
            }}
          >
            Открыть чат с ботом
          </Button>
        ) : null}
      </section>

      <section className="card stack stack--tight" aria-label="Экспорт">
        <h2 className="card__title">Календарь</h2>
        <p className="card__text">Файл .ics со сроками документов можно открыть в календаре телефона.</p>
        <CalendarExportButton organizationId={orgId} />
      </section>

      <section className="stack stack--tight" aria-label="Организация">
        {roleAllows(role, 'editor') ? (
          <Button size="medium" stretched variant="secondary" onClick={() => navigate(`/o/${orgId}/settings/profile`)}>
            Профиль организации
          </Button>
        ) : null}
        <Button size="medium" stretched variant="secondary" onClick={() => navigate('/account')}>
          Аккаунт
        </Button>
        {isOwner ? (
          <Button size="medium" stretched variant="destructive" onClick={() => setConfirm('delete-org')}>
            Удалить организацию
          </Button>
        ) : (
          <Button size="medium" stretched variant="destructive" onClick={() => setConfirm('leave')}>
            Выйти из организации
          </Button>
        )}
      </section>

      {notice ? (
        <p className="muted" aria-live="polite">
          {notice}
        </p>
      ) : null}

      <ConfirmDialog
        open={confirm !== null}
        title={confirm === 'delete-org' ? 'Удалить организацию?' : 'Выйти из организации?'}
        description={
          confirm === 'delete-org'
            ? 'Будут удалены все документы, напоминания и приглашения организации.'
            : 'Вы перестанете видеть документы и получать напоминания этой организации.'
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
