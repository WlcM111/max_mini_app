import {
  client,
  run,
  type Catalog,
  type Me,
  type Organization,
  type ProfileMatch,
  type OrganizationCreate,
  type OrganizationUpdate,
  type Suggestion,
} from '../../api/client';

/** Пользователь, его организации и состояние канала напоминаний. */
export const getMe = (): Promise<Me> => run(({ headers, signal }) => client.GET('/me', { headers, signal }));

/** Справочник видов деятельности, регионов, признаков и типов документов. */
export const getCatalog = (): Promise<Catalog> =>
  run(({ headers, signal }) => client.GET('/catalog', { headers, signal }));

export const getOrganization = (organizationId: string): Promise<Organization> =>
  run(({ headers, signal }) =>
    client.GET('/organizations/{organizationId}', { params: { path: { organizationId } }, headers, signal }),
  );

export const createOrganization = (body: OrganizationCreate): Promise<Organization> =>
  run(({ headers, signal }) => client.POST('/organizations', { body, headers, signal }));

export const updateOrganization = (organizationId: string, body: OrganizationUpdate): Promise<Organization> =>
  run(({ headers, signal }) =>
    client.PATCH('/organizations/{organizationId}', {
      params: { path: { organizationId } },
      body,
      headers,
      signal,
    }),
  );

export const deleteOrganization = (organizationId: string): Promise<void> =>
  run(({ headers, signal }) =>
    client.DELETE('/organizations/{organizationId}', { params: { path: { organizationId } }, headers, signal }),
  ).then(() => undefined);

export const listSuggestions = (organizationId: string): Promise<Suggestion[]> =>
  run(({ headers, signal }) =>
    client.GET('/organizations/{organizationId}/suggestions', {
      params: { path: { organizationId } },
      headers,
      signal,
    }),
  ).then((result) => result.items);

/**
 * Подбор вида деятельности и признаков по свободному описанию бизнеса (FR-22).
 * Возвращает только коды справочника; организацию не создаёт и не изменяет.
 */
export const matchProfile = (description: string): Promise<ProfileMatch> =>
  run(({ headers, signal }) => client.POST('/profile-match', { body: { description }, headers, signal }));
