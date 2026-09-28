import { useEffect, useMemo, type ReactNode } from 'react';
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError, NetworkError } from '../api/errors';
import type { Me } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { getMe } from '../features/organizations/api';
import { SessionContext } from '../session/useSession';
import type { SessionState } from '../session/sessionStore';
import type { MaxBridge } from '../platform/max/types';
import { Toaster } from '../shared/ui/Toast';

/** Общие правила серверного состояния (frontend-architecture §6). */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: true,
        retry: (failureCount: number, error: Error) => {
          if (failureCount >= 2) return false;
          if (error instanceof NetworkError) return true;
          if (error instanceof ApiError) return error.status === 429 || error.status >= 500;
          return false;
        },
        retryDelay: (attempt: number) => (attempt === 0 ? 500 : 1500),
      },
      mutations: { retry: false },
    },
  });
}

function SessionProvider({
  session,
  initialMe,
  bridge,
  children,
}: {
  session: SessionState;
  initialMe: Me;
  bridge: MaxBridge;
  children: ReactNode;
}) {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: queryKeys.me(), queryFn: getMe, initialData: initialMe });
  const value = useMemo(
    () => ({
      session,
      me: data,
      platform: bridge.platform,
      bridgeKind: bridge.kind,
      refreshMe: async () => {
        await queryClient.invalidateQueries({ queryKey: queryKeys.me() });
      },
    }),
    [session, data, bridge.platform, bridge.kind, queryClient],
  );
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

interface Props {
  session: SessionState;
  me: Me;
  bridge: MaxBridge;
  queryClient: QueryClient;
  children: ReactNode;
}

/** Корневые провайдеры: кэш запросов, сессия, уведомления. Платформа — атрибут для стилей. */
export function AppProviders({ session, me, bridge, queryClient, children }: Props) {
  useEffect(() => {
    document.documentElement.dataset.platform = bridge.platform;
  }, [bridge.platform]);
  return (
    <QueryClientProvider client={queryClient}>
      <SessionProvider session={session} initialMe={me} bridge={bridge}>
        {children}
        <Toaster />
      </SessionProvider>
    </QueryClientProvider>
  );
}
