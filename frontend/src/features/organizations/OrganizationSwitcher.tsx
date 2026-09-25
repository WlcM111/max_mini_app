import { useNavigate } from 'react-router';
import { useSession } from '../../session/useSession';
import { setLastOrganization } from '../../session/sessionStore';

/** Переключатель организаций из списка участий GET /me. */
export function OrganizationSwitcher({ organizationId }: { organizationId: string }) {
  const { me } = useSession();
  const navigate = useNavigate();
  if (me.memberships.length <= 1) return null;
  return (
    <div className="field">
      <label className="field__label" htmlFor="org-switcher">
        Организация
      </label>
      <select
        id="org-switcher"
        value={organizationId}
        onChange={(event) => {
          const next = event.target.value;
          setLastOrganization(next);
          navigate(`/o/${next}`, { replace: true });
        }}
      >
        {me.memberships.map((item) => (
          <option key={item.organization_id} value={item.organization_id}>
            {item.organization_name}
          </option>
        ))}
      </select>
    </div>
  );
}
