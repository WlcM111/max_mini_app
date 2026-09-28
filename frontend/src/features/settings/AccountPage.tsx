import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { deleteCurrentSession } from '../../api/client';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Avatar } from '../../shared/ui/Avatar';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { ModelDataBadge } from '../../shared/ui/ModelDataBadge';
import { KeyValue, NavRow } from '../../shared/ui/Rows';
import { toast } from '../../shared/ui/Toast';
import { initials, platformTitle } from '../../shared/lib/format';
import { clearLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
import { deleteAccount } from './api';

/** Аккаунт: сведения о пользователе, выход и удаление данных (FR-15, NFR-06). */
export function AccountPage() {
  const { me, platform } = useSession();
  const [confirm, setConfirm] = useState<'logout' | 'delete' | null>(null);

  const finish = () => {
    clearLastOrganization();
    window.location.reload();
  };
  const logout = useMutation({
    mutationFn: () => deleteCurrentSession(),
    onSuccess: finish,
    onError: (error) => {
      setConfirm(null);
      toast(messageForError(error), 'error');
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteAccount(),
    onSuccess: finish,
    onError: (error) => {
      setConfirm(null);
      toast(messageForError(error), 'error');
    },
  });

  const fullName = [me.account.first_name, me.account.last_name].filter(Boolean).join(' ');

  return (
    <AppShell title="Аккаунт">
      <section className="profile-card">
        <Avatar text={initials(me.account.first_name, me.account.last_name)} seed={me.account.id} size="l" />
        <div className="profile-card__text">
          <p className="profile-card__name">{fullName}</p>
          <p className="profile-card__meta">Профиль MAX</p>
        </div>
      </section>
      <section className="group" aria-labelledby="account-info-title">
        <h2 className="group__title" id="account-info-title">
          Сведения
        </h2>
        <div className="group__card">
          <KeyValue label="Организаций" value={`${me.memberships.length} из ${me.limits.max_organizations}`} />
          <KeyValue label="Платформа" value={platformTitle(platform)} />
          <KeyValue label="Версия приложения" value={import.meta.env.VITE_APP_VERSION ?? 'dev'} />
        </div>
      </section>
      <ModelDataBadge />
      <section className="group" aria-label="Выход и удаление">
        <div className="group__card">
          <NavRow icon="logout" title="Выйти" hint="Снова войти можно из чата с ботом" chevron={false} onClick={() => setConfirm('logout')} />
          <NavRow
            icon="trash"
            title="Удалить аккаунт"
            hint="Удалятся ваши данные и организации, где вы владелец"
            danger
            chevron={false}
            onClick={() => setConfirm('delete')}
          />
        </div>
      </section>
      <ConfirmDialog
        open={confirm !== null}
        title={confirm === 'delete' ? 'Удалить аккаунт?' : 'Выйти из аккаунта?'}
        description={
          confirm === 'delete'
            ? 'Удалятся ваши данные и организации, в которых вы владелец. Действие нельзя отменить.'
            : 'Сессия на этом устройстве будет завершена.'
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
