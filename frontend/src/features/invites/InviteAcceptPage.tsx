import { useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useLocation, useNavigate } from 'react-router';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { formatDate } from '../../shared/lib/dates';
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
        <p className="card__text">Ссылка приглашения недействительна. Попросите отправить новую.</p>
        <Button size="medium" variant="secondary" onClick={() => navigate('/', { replace: true })}>
          На главную
        </Button>
      </AppShell>
    );
  }

  return (
    <AppShell title="Приглашение">
      {preview.isLoading ? <LoadingView rows={2} /> : null}
      {preview.isError ? (
        <>
          <ErrorView error={preview.error} title="Приглашение недействительно" />
          <Button size="medium" variant="secondary" onClick={() => navigate('/', { replace: true })}>
            На главную
          </Button>
        </>
      ) : null}

      {preview.data ? (
        <>
          <div className="card stack stack--tight">
            <h2 className="card__title">{preview.data.organization_name}</h2>
            <p className="card__text">
              {preview.data.inviter_first_name ? `${preview.data.inviter_first_name} приглашает вас` : 'Вас приглашают'}{' '}
              как {ROLE_TITLES[preview.data.role] ?? preview.data.role}.
            </p>
            <p className="muted">Действительно до {formatDate(preview.data.expires_at.slice(0, 10))}</p>
          </div>
          <Button size="large" stretched loading={accept.isPending} onClick={() => accept.mutate()}>
            Принять приглашение
          </Button>
          <Button size="medium" stretched variant="secondary" onClick={() => navigate('/', { replace: true })}>
            Отклонить
          </Button>
        </>
      ) : null}

      {notice ? (
        <p className="field__error" role="alert" aria-live="polite">
          {notice}
        </p>
      ) : null}
    </AppShell>
  );
}
