import { Suspense, lazy, useEffect, type ReactNode } from 'react';
import { Outlet, RouterProvider, createMemoryRouter, useLocation, useNavigate, Navigate } from 'react-router';
import { hideBackButton, showBackButton } from '../platform/max/backButton';
import { getLastOrganization } from '../session/sessionStore';
import { useSession } from '../session/useSession';
import { LoadingView } from '../shared/ui/StateViews';
import { AppShell } from '../shared/ui/AppShell';
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
const FeaturesPage = lazy(() =>
  import('../features/onboarding/FeaturesPage').then((m) => ({ default: m.FeaturesPage })),
);
const SuggestionsPage = lazy(() =>
  import('../features/onboarding/SuggestionsPage').then((m) => ({ default: m.SuggestionsPage })),
);
const DatesPage = lazy(() => import('../features/onboarding/DatesPage').then((m) => ({ default: m.DatesPage })));
const DocumentFormPage = lazy(() =>
  import('../features/documents/DocumentFormPage').then((m) => ({ default: m.DocumentFormPage })),
);
const RenewPage = lazy(() => import('../features/documents/RenewPage').then((m) => ({ default: m.RenewPage })));
const MembersPage = lazy(() => import('../features/members/MembersPage').then((m) => ({ default: m.MembersPage })));
const InvitePage = lazy(() => import('../features/members/InvitePage').then((m) => ({ default: m.InvitePage })));
const InviteAcceptPage = lazy(() =>
  import('../features/invites/InviteAcceptPage').then((m) => ({ default: m.InviteAcceptPage })),
);
const SettingsPage = lazy(() => import('../features/settings/SettingsPage').then((m) => ({ default: m.SettingsPage })));
const AccountPage = lazy(() => import('../features/settings/AccountPage').then((m) => ({ default: m.AccountPage })));
const OrganizationSettingsPage = lazy(() =>
  import('../features/organizations/OrganizationSettingsPage').then((m) => ({ default: m.OrganizationSettingsPage })),
);

function PageFallback() {
  return (
    <AppShell title="Загрузка">
      <LoadingView rows={3} />
    </AppShell>
  );
}

function Lazy({ children }: { children: ReactNode }) {
  return <Suspense fallback={<PageFallback />}>{children}</Suspense>;
}

/** Корневой макет: связывает системную кнопку «Назад» MAX с историей маршрутов. */
function RootLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const isRoot = ROOT_PATTERNS.some((pattern) => pattern.test(location.pathname));

  useEffect(() => {
    if (isRoot) {
      void hideBackButton();
      return;
    }
    void showBackButton(() => {
      // Диплинк открывается без истории: возвращаемся на главный экран.
      if (location.key && location.key !== 'default') navigate(-1);
      else navigate('/', { replace: true });
    });
    return () => {
      void hideBackButton();
    };
  }, [isRoot, location.key, location.pathname, navigate]);

  return <Outlet />;
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
  const entry =
    inviteToken !== null ? { pathname: initialPath, state: { token: inviteToken } } : { pathname: initialPath };
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

/** Подключает маршрутизатор к приложению. */
export function AppRouter({ initialPath, inviteToken }: { initialPath: string; inviteToken: string | null }) {
  const router = createAppRouter(initialPath, inviteToken);
  return <RouterProvider router={router} />;
}
