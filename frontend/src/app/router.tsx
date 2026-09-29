import { Suspense, lazy, useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { Navigate, Outlet, RouterProvider, createMemoryRouter, useLocation, useNavigate, useNavigationType } from 'react-router';
import { hasSystemBackButton, hideBackButton, showBackButton } from '../platform/max/backButton';
import { getLastOrganization } from '../session/sessionStore';
import { roleAllows, useRole, useSession } from '../session/useSession';
import { initials } from '../shared/lib/format';
import { setNavDirection } from '../shared/lib/navDirection';
import { AppShell } from '../shared/ui/AppShell';
import { BackContext } from '../shared/ui/backContext';
import { LoadingView } from '../shared/ui/StateViews';
import { TabBar, type TabId } from '../shared/ui/TabBar';
import { DashboardPage } from '../features/organizations/DashboardPage';
import { DocumentsPage } from '../features/documents/DocumentsPage';
import { DocumentCardPage } from '../features/documents/DocumentCardPage';
import { NotFoundPage } from '../features/system/NotFoundPage';
import { ROOT_PATTERNS } from './routes';

// Экраны второго плана загружаются отдельными чанками (архитектура §12).
const WelcomePage = lazy(() => import('../features/onboarding/WelcomePage').then((m) => ({ default: m.WelcomePage })));
const OrganizationFormPage = lazy(() =>
  import('../features/onboarding/OrganizationFormPage').then((m) => ({ default: m.OrganizationFormPage })),
);
const FeaturesPage = lazy(() => import('../features/onboarding/FeaturesPage').then((m) => ({ default: m.FeaturesPage })));
const SuggestionsPage = lazy(() => import('../features/onboarding/SuggestionsPage').then((m) => ({ default: m.SuggestionsPage })));
const DatesPage = lazy(() => import('../features/onboarding/DatesPage').then((m) => ({ default: m.DatesPage })));
const DocumentFormPage = lazy(() => import('../features/documents/DocumentFormPage').then((m) => ({ default: m.DocumentFormPage })));
const ImportPage = lazy(() => import('../features/documents/ImportPage').then((m) => ({ default: m.ImportPage })));
const RenewPage = lazy(() => import('../features/documents/RenewPage').then((m) => ({ default: m.RenewPage })));
const MembersPage = lazy(() => import('../features/members/MembersPage').then((m) => ({ default: m.MembersPage })));
const InvitePage = lazy(() => import('../features/members/InvitePage').then((m) => ({ default: m.InvitePage })));
const InviteAcceptPage = lazy(() => import('../features/invites/InviteAcceptPage').then((m) => ({ default: m.InviteAcceptPage })));
const SettingsPage = lazy(() => import('../features/settings/SettingsPage').then((m) => ({ default: m.SettingsPage })));
const AccountPage = lazy(() => import('../features/settings/AccountPage').then((m) => ({ default: m.AccountPage })));
const OrganizationSettingsPage = lazy(() =>
  import('../features/organizations/OrganizationSettingsPage').then((m) => ({ default: m.OrganizationSettingsPage })),
);

// Разделы с нижней навигацией: главная, документы, участники, настройки.
const TAB_PATTERN = /^\/o\/([^/]+)(?:\/(documents|members|settings))?$/;

function PageFallback() {
  return (
    <AppShell title="Загрузка" titleSkeleton>
      <LoadingView rows={3} />
    </AppShell>
  );
}

function Lazy({ children }: { children: ReactNode }) {
  return <Suspense fallback={<PageFallback />}>{children}</Suspense>;
}

/**
 * Корневой макет: системная кнопка «Назад» MAX, нижняя навигация,
 * направление анимации переходов и восстановление прокрутки.
 */
function RootLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const navigationType = useNavigationType();
  const { me } = useSession();
  const isRoot = ROOT_PATTERNS.some((pattern) => pattern.test(location.pathname));
  const tabMatch = TAB_PATTERN.exec(location.pathname);
  const tabRoot: TabId | null = tabMatch ? ((tabMatch[2] as TabId | undefined) ?? 'home') : null;
  // На компьютере меню видно и на вложенных экранах: раздел определяется по адресу.
  const activeTab: TabId | null = tabRoot ?? sectionOf(location.pathname);
  const orgId = tabMatch?.[1] ?? /^\/o\/([^/]+)/.exec(location.pathname)?.[1] ?? getLastOrganization() ?? me.memberships[0]?.organization_id ?? '';
  const hasOrg = me.memberships.some((item) => item.organization_id === orgId);
  const navMode = tabRoot ? 'tabs' : activeTab && hasOrg ? 'inner' : 'none';
  const canAdd = roleAllows(useRole(orgId), 'editor');

  // Направление перехода задаётся до рендера нового экрана.
  const previous = useRef<{ key: string; tab: TabId | null }>({ key: location.key, tab: tabRoot });
  if (previous.current.key !== location.key) {
    const tabSwitch = previous.current.tab !== null && tabRoot !== null;
    setNavDirection(tabSwitch ? 'fade' : navigationType === 'POP' ? 'back' : navigationType === 'PUSH' ? 'forward' : 'fade');
    previous.current = { key: location.key, tab: tabRoot };
  }

  // Позиция прокрутки запоминается для каждого экрана и возвращается при «Назад».
  const positions = useRef(new Map<string, number>());
  const currentKey = useRef(location.key);
  useEffect(() => {
    const onScroll = () => positions.current.set(currentKey.current, window.scrollY);
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);
  useLayoutEffect(() => {
    if (currentKey.current === location.key) return;
    currentKey.current = location.key;
    const saved = navigationType === 'POP' ? positions.current.get(location.key) : undefined;
    window.scrollTo(0, saved ?? 0);
  }, [location.key, navigationType]);

  const [systemBack, setSystemBack] = useState(true);
  useEffect(() => {
    void hasSystemBackButton().then(setSystemBack);
  }, []);

  const goBack = useCallback(() => {
    // Диплинк открывается без истории: возвращаемся на главный экран.
    if (location.key && location.key !== 'default') navigate(-1);
    else navigate('/', { replace: true });
  }, [location.key, navigate]);

  useEffect(() => {
    if (isRoot) {
      void hideBackButton();
      return;
    }
    void showBackButton(goBack);
  }, [isRoot, goBack]);
  useEffect(
    () => () => {
      void hideBackButton();
    },
    [],
  );

  const selectTab = (tab: TabId) => {
    if (!orgId) return;
    const target = tab === 'home' ? `/o/${orgId}` : `/o/${orgId}/${tab}`;
    if (!tabRoot) {
      navigate(target);
      return;
    }
    if (tab === tabRoot) {
      window.scrollTo({ top: 0, behavior: 'smooth' });
      return;
    }
    // Разделы не копят историю: «Назад» из любого раздела ведёт на главную.
    const fromHome = (location.state as { fromHome?: boolean } | null)?.fromHome === true;
    if (tab === 'home') {
      if (fromHome) navigate(-1);
      else navigate(target, { replace: true });
      return;
    }
    if (tabRoot === 'home') navigate(target, { state: { fromHome: true } });
    else navigate(target, { replace: true, state: { fromHome } });
  };

  const organizationName = me.memberships.find((item) => item.organization_id === orgId)?.organization_name;
  const userName = [me.account.first_name, me.account.last_name].filter(Boolean).join(' ');

  return (
    <BackContext.Provider value={!systemBack && !isRoot ? goBack : null}>
      <div className="app-root" data-nav={navMode}>
        <Outlet />
        {navMode !== 'none' ? (
          <TabBar
            active={activeTab}
            organizationName={organizationName}
            userName={userName}
            userInitials={initials(me.account.first_name, me.account.last_name)}
            userSeed={me.account.id}
            canAdd={canAdd}
            onSelect={selectTab}
            onAdd={() => navigate(`/o/${orgId}/documents/new`)}
            onAccount={() => navigate('/account')}
            onHome={() => navigate(`/o/${orgId}`)}
          />
        ) : null}
      </div>
    </BackContext.Provider>
  );
}

// Раздел вложенного экрана — для бокового меню на компьютере.
function sectionOf(pathname: string): TabId | null {
  if (/^\/o\/[^/]+\/documents\//.test(pathname) || /^\/d\//.test(pathname)) return 'documents';
  if (/^\/o\/[^/]+\/(members|invite)/.test(pathname)) return 'members';
  if (/^\/o\/[^/]+\/settings\//.test(pathname) || pathname === '/account') return 'settings';
  return null;
}

/** Главный экран выбирается по последней организации пользователя. */
function HomeRedirect() {
  const { me } = useSession();
  if (me.memberships.length === 0) return <Navigate to="/welcome" replace />;
  const last = getLastOrganization();
  const membership = me.memberships.find((item) => item.organization_id === last) ?? me.memberships[0];
  return <Navigate to={membership ? `/o/${membership.organization_id}` : '/welcome'} replace />;
}

/** Создаёт маршрутизатор в памяти: адресная строка MAX не используется (ADR-003). */
export function createAppRouter(initialPath: string, inviteToken: string | null) {
  const entry = inviteToken !== null ? { pathname: initialPath, state: { token: inviteToken } } : { pathname: initialPath };
  return createMemoryRouter(
    [
      {
        path: '/',
        element: <RootLayout />,
        children: [
          { index: true, element: <HomeRedirect /> },
          { path: 'welcome', element: <Lazy><WelcomePage /></Lazy> },
          { path: 'onboarding/organization', element: <Lazy><OrganizationFormPage /></Lazy> },
          { path: 'onboarding/features', element: <Lazy><FeaturesPage /></Lazy> },
          { path: 'onboarding/suggestions', element: <Lazy><SuggestionsPage /></Lazy> },
          { path: 'onboarding/dates', element: <Lazy><DatesPage /></Lazy> },
          { path: 'o/:orgId', element: <DashboardPage /> },
          { path: 'o/:orgId/documents', element: <DocumentsPage /> },
          { path: 'o/:orgId/documents/import', element: <Lazy><ImportPage /></Lazy> },
          { path: 'o/:orgId/documents/typical', element: <Lazy><SuggestionsPage /></Lazy> },
          { path: 'o/:orgId/documents/typical/dates', element: <Lazy><DatesPage /></Lazy> },
          { path: 'o/:orgId/documents/new', element: <Lazy><DocumentFormPage mode="create" /></Lazy> },
          { path: 'o/:orgId/members', element: <Lazy><MembersPage /></Lazy> },
          { path: 'o/:orgId/invite', element: <Lazy><InvitePage /></Lazy> },
          { path: 'o/:orgId/settings', element: <Lazy><SettingsPage /></Lazy> },
          { path: 'o/:orgId/settings/profile', element: <Lazy><OrganizationSettingsPage /></Lazy> },
          { path: 'd/:docId', element: <DocumentCardPage /> },
          { path: 'd/:docId/edit', element: <Lazy><DocumentFormPage mode="edit" /></Lazy> },
          { path: 'd/:docId/renew', element: <Lazy><RenewPage /></Lazy> },
          { path: 'account', element: <Lazy><AccountPage /></Lazy> },
          { path: 'invite', element: <Lazy><InviteAcceptPage /></Lazy> },
          { path: '*', element: <NotFoundPage /> },
        ],
      },
    ],
    { initialEntries: [entry] },
  );
}

/** Подключает маршрутизатор; экземпляр создаётся один раз (не на каждый рендер). */
export function AppRouter({ initialPath, inviteToken }: { initialPath: string; inviteToken: string | null }) {
  const [router] = useState(() => createAppRouter(initialPath, inviteToken));
  return <RouterProvider router={router} />;
}
