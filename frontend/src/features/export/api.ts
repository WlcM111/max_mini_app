import { client, run, type CalendarExport } from '../../api/client';

/** Готовит одноразовую ссылку на файл календаря (действует 10 минут). */
export const createCalendarExport = (organizationId: string): Promise<CalendarExport> =>
  run(({ headers, signal }) =>
    client.POST('/organizations/{organizationId}/exports/calendar', {
      params: { path: { organizationId } },
      headers,
      signal,
    }),
  );
