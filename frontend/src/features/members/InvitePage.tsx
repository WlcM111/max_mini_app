import { useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router';
import type { InviteCreated } from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Banner } from '../../shared/ui/Banner';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';
import { toast } from '../../shared/ui/Toast';
import { uuidV4 } from '../../shared/lib/uuid';
import { haptic } from '../../platform/max/haptics';
import { copyToClipboard } from '../../platform/max/links';
import { shareInvite } from '../../platform/max/share';
import { useSession } from '../../session/useSession';
import { createInvite } from './api';

const ROLES = [
  { value: 'editor', title: 'Редактор', text: 'Добавляет, изменяет и продлевает документы' },
  { value: 'viewer', title: 'Наблюдатель', text: 'Только просматривает документы и сроки' },
] as const;

/** Приглашение участника: создание одноразовой ссылки и передача её в MAX (FR-10). */
export function InvitePage() {
  const { orgId = '' } = useParams();
  const queryClient = useQueryClient();
  const { me } = useSession();
  const inviteId = useRef(uuidV4());
  const [role, setRole] = useState<'editor' | 'viewer'>('editor');
  const [invite, setInvite] = useState<InviteCreated | null>(null);
  const organizationName = me.memberships.find((item) => item.organization_id === orgId)?.organization_name;

  const create = useMutation({
    mutationFn: () => createInvite(orgId, inviteId.current, role),
    onSuccess: async (data) => {
      setInvite(data);
      void haptic('success');
      await queryClient.invalidateQueries({ queryKey: queryKeys.invites(orgId) });
    },
    onError: (error) => {
      inviteId.current = uuidV4();
      toast(messageForError(error), 'error');
    },
  });

  const share = () => {
    if (!invite) return;
    void shareInvite(invite.share_text, invite.link_url).then((outcome) => {
      if (outcome === 'copied') toast('Ссылка скопирована — отправьте её в чате MAX', 'success');
      if (outcome === 'failed') toast('Не удалось поделиться: скопируйте ссылку вручную', 'error');
    });
  };
  const copy = () => {
    if (!invite) return;
    void copyToClipboard(invite.link_url).then((copied) =>
      toast(copied ? 'Ссылка скопирована' : 'Не удалось скопировать — выделите ссылку вручную', copied ? 'success' : 'error'),
    );
  };

  return (
    <AppShell
      title="Пригласить участника"
      subtitle={organizationName}
      actions={
        invite ? (
          <>
            <Button size="l" stretched icon="send" onClick={share}>
              Отправить в MAX
            </Button>
            <Button variant="neutral" stretched icon="copy" onClick={copy}>
              Скопировать ссылку
            </Button>
          </>
        ) : (
          <Button size="l" stretched loading={create.isPending} onClick={() => create.mutate()}>
            Создать приглашение
          </Button>
        )
      }
    >
      <section className="group" aria-labelledby="invite-role-title">
        <h2 className="group__title" id="invite-role-title">
          Роль участника
        </h2>
        <div className="choices" role="radiogroup" aria-labelledby="invite-role-title">
          {ROLES.map((item) => (
            <label
              key={item.value}
              className="choice"
              data-checked={role === item.value ? 'true' : 'false'}
              data-disabled={invite ? 'true' : 'false'}
            >
              <input
                type="radio"
                name="invite-role"
                className="choice__input"
                value={item.value}
                checked={role === item.value}
                disabled={invite !== null}
                onChange={() => setRole(item.value)}
              />
              <span className="choice__radio" aria-hidden="true" />
              <span className="choice__body">
                <span className="choice__title">{item.title}</span>
                <span className="choice__text">{item.text}</span>
              </span>
            </label>
          ))}
        </div>
      </section>
      {invite ? (
        <section className="link-card" aria-labelledby="invite-ready-title">
          <div className="link-card__head">
            <span className="success-badge" aria-hidden="true">
              <Icon name="check" size={24} />
            </span>
            <div>
              <h2 className="link-card__title" id="invite-ready-title">
                Ссылка готова
              </h2>
              <p className="link-card__text">Действует 72 часа и срабатывает один раз</p>
            </div>
          </div>
          <p className="link-box">{invite.link_url}</p>
        </section>
      ) : (
        <Banner tone="info" icon="info">
          Ссылку покажем один раз — отправьте её сразу. Посмотреть её повторно нельзя, но можно создать новую.
        </Banner>
      )}
    </AppShell>
  );
}
