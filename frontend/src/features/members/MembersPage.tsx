import { useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { formatDate } from '../../shared/lib/dates';
import { roleAllows, useRole, useSession } from '../../session/useSession';
import { listInvites, listMembers, removeMember, revokeInvite, updateMemberRole } from './api';

const ROLE_TITLES: Record<string, string> = { owner: 'Владелец', editor: 'Редактор', viewer: 'Наблюдатель' };

/** Участники организации, их роли и активные приглашения (FR-10). */
export function MembersPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { refreshMe } = useSession();
  const role = useRole(orgId);
  const isOwner = roleAllows(role, 'owner');
  const [removing, setRemoving] = useState<{ accountId: string; self: boolean } | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const members = useQuery({
    queryKey: queryKeys.members(orgId),
    queryFn: () => listMembers(orgId),
    enabled: orgId !== '',
    staleTime: 60_000,
  });
  const invites = useQuery({
    queryKey: queryKeys.invites(orgId),
    queryFn: () => listInvites(orgId),
    enabled: orgId !== '' && isOwner,
    staleTime: 60_000,
  });

  const changeRole = useMutation({
    mutationFn: ({ accountId, nextRole }: { accountId: string; nextRole: 'editor' | 'viewer' }) =>
      updateMemberRole(orgId, accountId, nextRole),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.members(orgId) });
    },
    onError: (error) => setNotice(messageForError(error)),
  });

  const remove = useMutation({
    mutationFn: (accountId: string) => removeMember(orgId, accountId),
    onSuccess: async (_data, accountId) => {
      const self = removing?.self ?? false;
      setRemoving(null);
      await queryClient.invalidateQueries({ queryKey: queryKeys.members(orgId) });
      await refreshMe();
      if (self) navigate('/', { replace: true });
      else setNotice('Участник исключён');
      void accountId;
    },
    onError: (error) => {
      setRemoving(null);
      setNotice(messageForError(error));
    },
  });

  const revoke = useMutation({
    mutationFn: (inviteId: string) => revokeInvite(inviteId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.invites(orgId) });
      setNotice('Приглашение отозвано');
    },
    onError: (error) => setNotice(messageForError(error)),
  });

  return (
    <AppShell
      title="Участники"
      actions={
        isOwner ? (
          <Button size="large" stretched onClick={() => navigate(`/o/${orgId}/invite`)}>
            Пригласить
          </Button>
        ) : null
      }
    >
      {members.isLoading ? <LoadingView rows={3} /> : null}
      {members.isError ? <ErrorView error={members.error} onRetry={() => void members.refetch()} /> : null}

      {members.data && members.data.length === 1 ? (
        <EmptyView
          title="Вы пока работаете один"
          description={isOwner ? 'Пригласите коллегу — он увидит те же сроки.' : undefined}
        />
      ) : null}

      <div className="list">
        {members.data?.map((member) => (
          <div key={member.account_id} className="list-item" style={{ cursor: 'default' }}>
            <span className="grow">
              <span className="list-item__title truncate">
                {member.first_name} {member.last_name ?? ''} {member.is_me ? '(вы)' : ''}
              </span>
              <span className="list-item__meta">
                {ROLE_TITLES[member.role] ?? member.role} · с {formatDate(member.joined_at.slice(0, 10))}
              </span>
            </span>
            {isOwner && member.role !== 'owner' ? (
              <select
                aria-label={`Роль участника ${member.first_name}`}
                value={member.role}
                onChange={(event) =>
                  changeRole.mutate({
                    accountId: member.account_id,
                    nextRole: event.target.value as 'editor' | 'viewer',
                  })
                }
              >
                <option value="editor">Редактор</option>
                <option value="viewer">Наблюдатель</option>
              </select>
            ) : null}
            {(isOwner && member.role !== 'owner') || (member.is_me && member.role !== 'owner') ? (
              <Button
                size="small"
                variant="ghost"
                onClick={() => setRemoving({ accountId: member.account_id, self: member.is_me })}
              >
                {member.is_me ? 'Выйти' : 'Исключить'}
              </Button>
            ) : null}
          </div>
        ))}
      </div>

      {isOwner && invites.data && invites.data.length > 0 ? (
        <section className="stack stack--tight" aria-label="Активные приглашения">
          <h2 className="card__title">Активные приглашения</h2>
          {invites.data.map((invite) => (
            <div key={invite.id} className="list-item" style={{ cursor: 'default' }}>
              <span className="grow">
                <span className="list-item__title">{ROLE_TITLES[invite.role] ?? invite.role}</span>
                <span className="list-item__meta">действует до {formatDate(invite.expires_at.slice(0, 10))}</span>
              </span>
              <Button size="small" variant="ghost" onClick={() => revoke.mutate(invite.id)}>
                Отозвать
              </Button>
            </div>
          ))}
        </section>
      ) : null}

      {notice ? (
        <p className="muted" aria-live="polite">
          {notice}
        </p>
      ) : null}

      <ConfirmDialog
        open={removing !== null}
        title={removing?.self ? 'Выйти из организации?' : 'Исключить участника?'}
        description={
          removing?.self
            ? 'Вы перестанете видеть документы этой организации и получать напоминания.'
            : 'Участник потеряет доступ к документам организации.'
        }
        confirmLabel={removing?.self ? 'Выйти' : 'Исключить'}
        destructive
        pending={remove.isPending}
        onConfirm={() => {
          if (removing) remove.mutate(removing.accountId);
        }}
        onCancel={() => setRemoving(null)}
      />
    </AppShell>
  );
}
