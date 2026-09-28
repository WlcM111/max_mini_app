import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '../../api/queryKeys';
import { listMembers } from './api';

/** Имена участников по account_id; запрос уходит, только если он нужен (enabled). */
export function useMemberNames(organizationId: string, enabled: boolean): Map<string, string> {
  const members = useQuery({
    queryKey: queryKeys.members(organizationId),
    queryFn: () => listMembers(organizationId),
    enabled: enabled && organizationId !== '',
    staleTime: 60_000,
  });
  return new Map((members.data ?? []).map((m) => [m.account_id, [m.first_name, m.last_name].filter(Boolean).join(' ')]));
}
