import { useRef, useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import type { InviteCreated } from '../../api/client';
import { AppShell } from '../../shared/ui/AppShell';
import { uuidV4 } from '../../shared/lib/uuid';
import { copyToClipboard } from '../../platform/max/links';
import { shareInvite } from '../../platform/max/share';
import { createInvite } from './api';

/** Приглашение участника: создание ссылки и передача её в MAX (FR-10). */
export function InvitePage() {
  const { orgId = '' } = useParams();
  const queryClient = useQueryClient();
  const inviteId = useRef(uuidV4());
  const [role, setRole] = useState<'editor' | 'viewer'>('editor');
  const [invite, setInvite] = useState<InviteCreated | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () => createInvite(orgId, inviteId.current, role),
    onSuccess: async (data) => {
      setInvite(data);
      setNotice('Ссылка действует 72 часа и работает один раз');
      await queryClient.invalidateQueries({ queryKey: queryKeys.invites(orgId) });
    },
    onError: (error) => {
      inviteId.current = uuidV4();
      setNotice(messageForError(error));
    },
  });

  return (
    <AppShell
      title="Пригласить участника"
      actions={
        invite ? (
          <>
            <Button
              size="large"
              stretched
              onClick={() => {
                void shareInvite(invite.share_text, invite.link_url).then((outcome) => {
                  if (outcome === 'copied') setNotice('Ссылка скопирована — отправьте её в чате MAX');
                  if (outcome === 'failed') setNotice('Не удалось поделиться: скопируйте ссылку вручную');
                });
              }}
            >
              Отправить в MAX
            </Button>
            <Button
              size="medium"
              stretched
              variant="secondary"
              onClick={() => {
                void copyToClipboard(invite.link_url).then((copied) =>
                  setNotice(copied ? 'Ссылка скопирована' : 'Скопируйте ссылку вручную'),
                );
              }}
            >
              Скопировать ссылку
            </Button>
          </>
        ) : (
          <Button size="large" stretched loading={create.isPending} onClick={() => create.mutate()}>
            Создать приглашение
          </Button>
        )
      }
    >
      <div className="field">
        <label className="field__label" htmlFor="invite-role">
          Роль участника
        </label>
        <select
          id="invite-role"
          value={role}
          disabled={invite !== null}
          onChange={(event) => setRole(event.target.value as 'editor' | 'viewer')}
        >
          <option value="editor">Редактор — добавляет и изменяет документы</option>
          <option value="viewer">Наблюдатель — только смотрит</option>
        </select>
      </div>

      {invite ? (
        <div className="card stack stack--tight">
          <p className="card__text">{invite.share_text}</p>
          <p className="muted" style={{ wordBreak: 'break-all' }}>
            {invite.link_url}
          </p>
        </div>
      ) : (
        <p className="card__text">
          Ссылка выдаётся один раз: сохраните её сразу после создания. Повторно посмотреть ссылку нельзя.
        </p>
      )}

      {notice ? (
        <p className="muted" aria-live="polite">
          {notice}
        </p>
      ) : null}
    </AppShell>
  );
}
