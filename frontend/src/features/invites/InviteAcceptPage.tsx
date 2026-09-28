import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useLocation, useNavigate } from 'react-router';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Avatar } from '../../shared/ui/Avatar';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { formatDate } from '../../shared/lib/dates';
import { nameInitial } from '../../shared/lib/format';
import { setLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
import { acceptInvite, previewInvite } from './api';

const ROLE_TITLES: Record<string, string> = { editor: 'редактор', viewer: 'наблюдатель' };

/** Приём приглашения по диплинку inv_<токен> (FR-10). */
export function InviteAcceptPage({ token: tokenProp }: { token?: string }) {
  const location = useLocation();
  const navigate = useNavigate();
  const { refreshMe } = useSession();
  const [notice, setNotice] = useState<string | null>(null);
  const stateToken = (location.state as { token?: string } | null)?.token;
  const token = tokenProp ?? stateToken ?? '';
  const home = () => navigate('/', { replace: true });

  const preview = useQuery({
    queryKey: ['invite-preview', token],
    queryFn: () => previewInvite(token),
    enabled: token !== '',
    retry: false,
  });

  const accept = useMutation({
    mutationFn: () => acceptInvite(token),
    onSuccess: async (result) => {
      setLastOrganization(result.organization_id);
      await refreshMe();
      navigate(`/o/${result.organization_id}`, { replace: true });
    },
    onError: (error) => setNotice(messageForError(error)),
  });

  if (token === '') {
    return (
      <AppShell title="Приглашение">
        <EmptyView
          art="link"
          title="Ссылка приглашения недействительна"
          description="Попросите отправить новую ссылку."
          action={
            <Button variant="secondary" onClick={home}>
              На главную
            </Button>
          }
        />
      </AppShell>
    );
  }

  if (preview.isLoading) {
    return (
      <AppShell title="Приглашение">
        <LoadingView rows={1} variant="card" />
      </AppShell>
    );
  }

  if (preview.isError || !preview.data) {
    return (
      <AppShell title="Приглашение">
        <ErrorView
          error={preview.error}
          title="Приглашение недействительно"
          action={
            <Button variant="secondary" onClick={home}>
              На главную
            </Button>
          }
        />
      </AppShell>
    );
  }

  const data = preview.data;
  return (
    <AppShell
      title="Приглашение"
      actionsNote={
        notice ? (
          <p className="actionbar__note" role="alert">
            <Icon name="alert" size={18} />
            {notice}
          </p>
        ) : null
      }
      actions={
        <>
          <Button size="l" stretched loading={accept.isPending} onClick={() => accept.mutate()}>
            Принять приглашение
          </Button>
          <Button variant="tertiary" stretched disabled={accept.isPending} onClick={home}>
            Отклонить
          </Button>
        </>
      }
    >
      <section className="invite-card">
        <Avatar text={nameInitial(data.organization_name)} seed={data.organization_name} size="l" square />
        <h2 className="invite-card__org">{data.organization_name}</h2>
        <p className="invite-card__text">
          {data.inviter_first_name ? `${data.inviter_first_name} приглашает вас` : 'Вас приглашают'} в команду как{' '}
          {ROLE_TITLES[data.role] ?? data.role}.
        </p>
        <p className="invite-card__meta">
          <Icon name="clock" size={16} />
          Приглашение действует до {formatDate(data.expires_at.slice(0, 10))}
        </p>
      </section>
    </AppShell>
  );
}
