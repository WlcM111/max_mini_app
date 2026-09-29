import { beforeEach, describe, expect, it } from 'vitest';
import { clearSession, launchKeyOf, restoreSession, setLaunchContext, setSession, type SessionState } from './sessionStore';
import { me } from '../test/fixtures';

const session = (): SessionState => ({
  token: 'vvs_test',
  expiresAt: new Date(Date.now() + 3_600_000).toISOString(),
  account: me.account,
  start: { kind: 'none' },
});

const launch = (hash: string) => ({ initData: `auth_date=1&start_param=doc_x&hash=${hash}`, platform: 'web', appVersion: 'dev' });

describe('T-FE-SESSION: сессия привязана к запуску MAX', () => {
  beforeEach(() => clearSession());

  it('отпечаток запуска — подпись hash из initData', () => {
    expect(launchKeyOf('auth_date=1&hash=abc')).toBe('abc');
    expect(launchKeyOf('без подписи')).toBe('без подписи');
  });

  it('после перезагрузки страницы того же запуска сессия восстанавливается', () => {
    setLaunchContext(launch('first'));
    setSession(session());
    expect(restoreSession()?.token).toBe('vvs_test');
  });

  it('новое открытие из MAX (диплинк, приглашение) получает новую сессию', () => {
    setLaunchContext(launch('first'));
    setSession(session());
    setLaunchContext(launch('second'));
    expect(restoreSession()).toBeNull();
  });
});
