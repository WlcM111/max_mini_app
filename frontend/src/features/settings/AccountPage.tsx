import { useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { deleteCurrentSession } from '../../api/client';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { clearLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
import { deleteAccount } from './api';

/** Аккаунт: сведения о пользователе, выход и удаление данных (FR-15, NFR-06). */
export function AccountPage() {
  const navigate = useNavigate();
  const { me, platform } = useSession();
  const [confirm, setConfirm] = useState<'logout' | 'delete' | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const finish = () => {
    clearLastOrganization();
    window.location.reload();
  };

  const logout = useMutation({
    mutationFn: () => deleteCurrentSession(),
    onSuccess: finish,
    onError: (error) => {
      setConfirm(null);
      setNotice(messageForError(error));
    },
  });

  const remove = useMutation({
    mutationFn: () => deleteAccount(),
    onSuccess: finish,
    onError: (error) => {
      setConfirm(null);
      setNotice(messageForError(error));
    },
  });

  return (
    <AppShell title="Аккаунт" subtitle={me.account.first_name}>
      <section className="card stack stack--tight" aria-label="Сведения">
        <p className="card__text">
          <span className="muted">Имя: </span>
          {me.account.first_name} {me.account.last_name ?? ''}
        </p>
        <p className="card__text">
          <span className="muted">Организаций: </span>
          {me.memberships.length} из {me.limits.max_organizations}
        </p>
        <p className="card__text">
          <span className="muted">Платформа: </span>
          {platform}
        </p>
        <p className="card__text">
          <span className="muted">Версия приложения: </span>
          {import.meta.env.VITE_APP_VERSION ?? 'dev'}
        </p>
      </section>

      <ModelDataBadge />

      <Button size="medium" stretched variant="secondary" onClick={() => navigate(-1)}>
        Назад
      </Button>
      <Button size="medium" stretched variant="secondary" onClick={() => setConfirm('logout')}>
        Выйти
      </Button>
      <Button size="medium" stretched variant="destructive" onClick={() => setConfirm('delete')}>
        Удалить аккаунт
      </Button>

      {notice ? (
        <p className="field__error" role="alert" aria-live="polite">
          {notice}
        </p>
      ) : null}

      <ConfirmDialog
        open={confirm !== null}
        title={confirm === 'delete' ? 'Удалить аккаунт?' : 'Выйти из приложения?'}
        description={
          confirm === 'delete'
            ? 'Будут удалены ваши организации, документы и напоминания. Действие необратимо.'
            : 'Потребуется снова открыть приложение из чата с ботом.'
        }
        confirmLabel={confirm === 'delete' ? 'Удалить' : 'Выйти'}
        destructive={confirm === 'delete'}
        pending={logout.isPending || remove.isPending}
        onConfirm={() => (confirm === 'delete' ? remove.mutate() : logout.mutate())}
        onCancel={() => setConfirm(null)}
      />
    </AppShell>
  );
}
