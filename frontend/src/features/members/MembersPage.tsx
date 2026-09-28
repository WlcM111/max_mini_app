import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Avatar } from '../../shared/ui/Avatar';
import { Button } from '../../shared/ui/Button';
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog';
import { Fab } from '../../shared/ui/Fab';
import { Icon } from '../../shared/ui/Icon';
import { EmptyView, ErrorView, LoadingView } from '../../shared/ui/StateViews';
import { toast } from '../../shared/ui/Toast';
import { formatDate } from '../../shared/lib/dates';
import { initials, ROLE_TITLES } from '../../shared/lib/format';
import { roleAllows, useRole, useSession } from '../../session/useSession';
import { listInvites, listMembers, removeMember, revokeInvite, updateMemberRole } from './api';

const ROLE_OPTIONS = ['editor', 'viewer'] as const;

/** Участники организации, их роли и активные приглашения (FR-10). */
export function MembersPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { me, refreshMe } = useSession();
  const role = useRole(orgId);
  const isOwner = roleAllows(role, 'owner');
  const [removing, setRemoving] = useState<{ accountId: string; self: boolean } | null>(null);

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
      toast('Роль изменена', 'success');
    },
    onError: (error) => toast(messageForError(error), 'error'),
  });

  const remove = useMutation({
    mutationFn: (accountId: string) => removeMember(orgId, accountId),
    onSuccess: async () => {
      const self = removing?.self ?? false;
      setRemoving(null);
      await queryClient.invalidateQueries({ queryKey: queryKeys.members(orgId) });
      await refreshMe();
      if (self) navigate('/', { replace: true });
      else toast('Участник исключён', 'success');
    },
    onError: (error) => {
      setRemoving(null);
      toast(messageForError(error), 'error');
    },
  });

  const revoke = useMutation({
    mutationFn: (inviteId: string) => revokeInvite(inviteId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.invites(orgId) });
      toast('Приглашение отозвано', 'success');
    },
    onError: (error) => toast(messageForError(error), 'error'),
  });

  const list = members.data ?? [];
  const openInvites = invites.data ?? [];
  const organizationName = me.memberships.find((item) => item.organization_id === orgId)?.organization_name;

  return (
    <AppShell
      title="Участники"
      subtitle={organizationName}
      fab={isOwner ? <Fab icon="user-plus" label="Пригласить" onClick={() => navigate(`/o/${orgId}/invite`)} /> : null}
    >
      {members.isLoading ? <LoadingView rows={3} /> : null}
      {members.isError ? <ErrorView error={members.error} onRetry={() => void members.refetch()} /> : null}
      {list.length === 1 ? (
        <EmptyView
          art="people"
          title="Вы пока работаете один"
          description={isOwner ? 'Пригласите коллегу — он увидит те же сроки и сможет помогать с документами.' : undefined}
        />
      ) : null}

      {list.length > 0 ? (
        <section className="group" aria-labelledby="members-title">
          <h2 className="group__title" id="members-title">
            В команде: {list.length}
          </h2>
          <div className="group__card">
            {list.map((member) => {
              const fullName = [member.first_name, member.last_name].filter(Boolean).join(' ');
              const canManage = isOwner && member.role !== 'owner';
              return (
                <div key={member.account_id} className="member">
                  <div className="member__top">
                    <Avatar text={initials(member.first_name, member.last_name)} seed={member.account_id} />
                    <div className="member__info">
                      <span className="member__name">
                        {fullName}
                        {member.is_me ? <span className="member__you">вы</span> : null}
                      </span>
                      <span className="member__meta">
                        {ROLE_TITLES[member.role] ?? member.role}, в команде с {formatDate(member.joined_at.slice(0, 10))}
                      </span>
                    </div>
                  </div>
                  {canManage ? (
                    <div className="member__controls">
                      <div className="segmented" role="group" aria-label={`Роль участника ${fullName}`}>
                        {ROLE_OPTIONS.map((value) => (
                          <button
                            key={value}
                            type="button"
                            className="segmented__btn"
                            aria-pressed={member.role === value}
                            disabled={changeRole.isPending}
                            onClick={() => {
                              if (member.role !== value) changeRole.mutate({ accountId: member.account_id, nextRole: value });
                            }}
                          >
                            {ROLE_TITLES[value]}
                          </button>
                        ))}
                      </div>
                      <Button variant="danger-soft" size="s" onClick={() => setRemoving({ accountId: member.account_id, self: false })}>
                        Исключить
                      </Button>
                    </div>
                  ) : null}
                  {!isOwner && member.is_me ? (
                    <div className="member__controls">
                      <Button variant="danger-soft" size="s" icon="logout" onClick={() => setRemoving({ accountId: member.account_id, self: true })}>
                        Выйти из организации
                      </Button>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        </section>
      ) : null}

      {isOwner && openInvites.length > 0 ? (
        <section className="group" aria-labelledby="invites-title">
          <h2 className="group__title" id="invites-title">
            Активные приглашения
          </h2>
          <div className="group__card">
            {openInvites.map((invite) => (
              <div key={invite.id} className="invite-row">
                <span className="nav-row__icon" aria-hidden="true">
                  <Icon name="link" />
                </span>
                <span className="invite-row__text">
                  <span className="invite-row__title">{ROLE_TITLES[invite.role] ?? invite.role}</span>
                  <span className="invite-row__meta">Действует до {formatDate(invite.expires_at.slice(0, 10))}</span>
                </span>
                <Button variant="tertiary" size="s" disabled={revoke.isPending} onClick={() => revoke.mutate(invite.id)}>
                  Отозвать
                </Button>
              </div>
            ))}
          </div>
        </section>
      ) : null}

      <ConfirmDialog
        open={removing !== null}
        title={removing?.self ? 'Выйти из организации?' : 'Исключить участника?'}
        description={removing?.self ? 'Вы потеряете доступ к документам этой организации.' : 'Участник потеряет доступ к документам организации.'}
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
