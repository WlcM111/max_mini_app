import { useCallback, useEffect, useMemo, useState } from 'react';
import { AppShell } from '../shared/ui/AppShell';
import { LoadingView } from '../shared/ui/StateViews';
import { LaunchErrorPage } from '../features/system/LaunchErrorPage';
import { NotInMaxPage } from '../features/system/NotInMaxPage';
import { resetBridge } from '../platform/max/bridge';
import { bootstrap, type BootstrapOutcome } from './bootstrap';
import { AppProviders, createQueryClient } from './providers';
import { AppRouter } from './router';

type State = { status: 'loading' } | BootstrapOutcome;

/** Корневой компонент: запуск приложения и выбор первого экрана. */
export function App() {
  const [state, setState] = useState<State>({ status: 'loading' });
  const queryClient = useMemo(() => createQueryClient(), []);

  const start = useCallback(() => {
    setState({ status: 'loading' });
    resetBridge();
    void bootstrap().then(setState);
  }, []);

  useEffect(() => {
    start();
  }, [start]);

  if (state.status === 'loading') {
    return (
      <AppShell title="Вовремя">
        <LoadingView rows={3} label="Запуск приложения" />
      </AppShell>
    );
  }
  if (state.status === 'not-in-max') return <NotInMaxPage onRetry={start} />;
  if (state.status === 'launch-error') return <LaunchErrorPage message={state.message} onRetry={start} />;
  if (state.status === 'network-error') {
    return <LaunchErrorPage message={state.message} onRetry={start} retryAfterSeconds={state.retryAfterSeconds} />;
  }

  return (
    <AppProviders session={state.session} me={state.me} bridge={state.bridge} queryClient={queryClient}>
      <AppRouter initialPath={state.initialPath} inviteToken={state.inviteToken} />
    </AppProviders>
  );
}
