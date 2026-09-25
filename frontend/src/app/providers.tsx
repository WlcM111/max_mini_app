import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { MaxUI } from '@maxhub/max-ui';
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query';
import { NetworkError } from '../api/errors';
import { ApiError } from '../api/errors';
import type { Me } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { getMe } from '../features/organizations/api';
import { SessionContext } from '../session/useSession';
import type { SessionState } from '../session/sessionStore';
import type { MaxBridge } from '../platform/max/types';

/** Общие правила серверного состояния (frontend-architecture §6). */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: true,
        retry: (failureCount, error) => {
          if (failureCount >= 2) return false;
          if (error instanceof NetworkError) return true;
          if (error instanceof ApiError) return error.status === 429 || error.status >= 500;
          return false;
        },
        retryDelay: (attempt) => (attempt === 0 ? 500 : 1500),
      },
      mutations: { retry: false },
    },
  });
}

/** Тема MAX UI: схема из системной настройки, платформа из Bridge (§11). */
function useColorScheme(): 'light' | 'dark' {
  const [scheme, setScheme] = useState<'light' | 'dark'>(() =>
    typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
  );
  useEffect(() => {
    const media = window.matchMedia?.('(prefers-color-scheme: dark)');
    if (!media) return;
    const listener = (event: MediaQueryListEvent) => setScheme(event.matches ? 'dark' : 'light');
    media.addEventListener('change', listener);
    return () => media.removeEventListener('change', listener);
  }, []);
  return scheme;
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

/** Корневые провайдеры приложения: тема MAX, кэш запросов, сессия. */
export function AppProviders({ session, me, bridge, queryClient, children }: Props) {
  const colorScheme = useColorScheme();
  const platform = bridge.platform === 'ios' ? 'ios' : 'android';
  return (
    <MaxUI platform={platform} colorScheme={colorScheme}>
      <QueryClientProvider client={queryClient}>
        <SessionProvider session={session} initialMe={me} bridge={bridge}>
          {children}
        </SessionProvider>
      </QueryClientProvider>
    </MaxUI>
  );
}
