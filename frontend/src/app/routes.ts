/** Маршруты мини-приложения (карта экранов — frontend-architecture §3). */
export const routes = {
  home: '/',
  welcome: '/welcome',
  onboardingOrganization: '/onboarding/organization',
  onboardingFeatures: '/onboarding/features',
  onboardingSuggestions: '/onboarding/suggestions',
  onboardingDates: '/onboarding/dates',
  dashboard: (organizationId = ':orgId') => `/o/${organizationId}`,
  documents: (organizationId = ':orgId') => `/o/${organizationId}/documents`,
  documentNew: (organizationId = ':orgId') => `/o/${organizationId}/documents/new`,
  documentsImport: (organizationId = ':orgId') => `/o/${organizationId}/documents/import`,
  documentsTypical: (organizationId = ':orgId') => `/o/${organizationId}/documents/typical`,
  documentsTypicalDates: (organizationId = ':orgId') => `/o/${organizationId}/documents/typical/dates`,
  documentCard: (documentId = ':docId') => `/d/${documentId}`,
  documentEdit: (documentId = ':docId') => `/d/${documentId}/edit`,
  documentRenew: (documentId = ':docId') => `/d/${documentId}/renew`,
  members: (organizationId = ':orgId') => `/o/${organizationId}/members`,
  invite: (organizationId = ':orgId') => `/o/${organizationId}/invite`,
  settings: (organizationId = ':orgId') => `/o/${organizationId}/settings`,
  organizationProfile: (organizationId = ':orgId') => `/o/${organizationId}/settings/profile`,
  account: '/account',
  inviteAccept: '/invite',
  errorNotInMax: '/error/not-in-max',
  errorLaunch: '/error/launch',
} as const;

/** Корневые экраны: системная кнопка «Назад» на них не показывается. */
export const ROOT_PATTERNS: RegExp[] = [
  /^\/$/,
  /^\/welcome$/,
  /^\/o\/[^/]+$/,
  /^\/error\//,
];
