import { useNavigate } from 'react-router';
import { Avatar } from '../../shared/ui/Avatar';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';
import { Sheet } from '../../shared/ui/Sheet';
import { nameInitial, ROLE_TITLES } from '../../shared/lib/format';
import { setLastOrganization } from '../../session/sessionStore';
import { useSession } from '../../session/useSession';
import { resetDraft } from '../onboarding/onboardingDraft';

interface Props {
  open: boolean;
  organizationId: string;
  onClose: () => void;
}

/** Шторка выбора организации и создания новой. */
export function OrganizationSwitcher({ open, organizationId, onClose }: Props) {
  const navigate = useNavigate();
  const { me } = useSession();
  const atLimit = me.memberships.length >= me.limits.max_organizations;

  return (
    <Sheet open={open} onClose={onClose} title="Организации">
      <div className="org-list">
        {me.memberships.map((item) => {
          const current = item.organization_id === organizationId;
          return (
            <button
              key={item.organization_id}
              type="button"
              className="org-option"
              aria-current={current ? 'true' : undefined}
              onClick={() => {
                onClose();
                if (current) return;
                setLastOrganization(item.organization_id);
                navigate(`/o/${item.organization_id}`, { replace: true });
              }}
            >
              <Avatar text={nameInitial(item.organization_name)} seed={item.organization_id} square />
              <span className="org-option__text">
                <span className="org-option__name">{item.organization_name}</span>
                <span className="org-option__role">{ROLE_TITLES[item.role] ?? item.role}</span>
              </span>
              {current ? <Icon name="check" className="org-option__check" /> : null}
            </button>
          );
        })}
      </div>
      <Button
        variant="secondary"
        size="l"
        stretched
        icon="plus"
        disabled={atLimit}
        onClick={() => {
          onClose();
          resetDraft();
          navigate('/onboarding/organization');
        }}
      >
        Создать организацию
      </Button>
      {atLimit ? <p className="sheet__text">Можно состоять не более чем в {me.limits.max_organizations} организациях.</p> : null}
    </Sheet>
  );
}
