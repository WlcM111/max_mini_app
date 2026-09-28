import type { ReactNode } from 'react';
import { render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { SessionContext } from '../session/useSession';
import { setSession } from '../session/sessionStore';
import * as fixtures from './fixtures';

/** Тестовое окружение приложения: тема, кэш запросов и подтверждённая сессия. */
export function renderWithProviders(
  element: ReactNode,
  {
    route = '/',
    path = '/',
    state,
    me = fixtures.me,
  }: { route?: string; path?: string; state?: unknown; me?: typeof fixtures.me } = {},
) {
  setSession({
    token: fixtures.session.token,
    expiresAt: fixtures.session.expires_at,
    account: fixtures.account,
    start: { kind: 'none' },
  });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });
  const router = createMemoryRouter([{ path, element }], {
    initialEntries: [state ? { pathname: route, state } : { pathname: route }],
  });
  const value = {
    session: {
      token: fixtures.session.token,
      expiresAt: fixtures.session.expires_at,
      account: fixtures.account,
      start: { kind: 'none' as const },
    },
    me,
    platform: 'web',
    bridgeKind: 'mock' as const,
    refreshMe: async () => undefined,
  };
  return render(
      <QueryClientProvider client={queryClient}>
        <SessionContext.Provider value={value}>
          <RouterProvider router={router} />
        </SessionContext.Provider>
      </QueryClientProvider>,
  );
}
