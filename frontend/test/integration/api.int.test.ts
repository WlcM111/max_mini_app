import { beforeAll, describe, expect, it } from 'vitest';
import { createSession, deleteCurrentSession } from '../../src/api/client';
import { ApiError } from '../../src/api/errors';
import { clearSession, getSession, setLaunchContext } from '../../src/session/sessionStore';
import { uuidV4 } from '../../src/shared/lib/uuid';
import { shiftDate, todayInTimeZone } from '../../src/shared/lib/dates';
import {
  createOrganization,
  getCatalog,
  getMe,
  getOrganization,
  listSuggestions,
  updateOrganization,
} from '../../src/features/organizations/api';
import {
  createDocument,
  createDocumentsBatch,
  deleteDocument,
  getDocument,
  listDocuments,
  renewDocument,
  updateDocument,
} from '../../src/features/documents/api';
import { createInvite, listInvites, listMembers, removeMember, revokeInvite } from '../../src/features/members/api';
import { acceptInvite, previewInvite } from '../../src/features/invites/api';
import { getNotificationSettings, putNotificationSettings } from '../../src/features/settings/api';
import { createCalendarExport } from '../../src/features/export/api';
import { apiBase, botAdminBase, launchDataFor } from './launch';

// Код мини-приложения выполняется против настоящих core, reminders и bot:
// тот же API-клиент, те же функции экранов, реальные HTTP-запросы.

const OWNER_ID = 900_101;
const GUEST_ID = 900_202;
const ORG_ID = uuidV4();
const DOC_ID = uuidV4();

async function loginAs(userId: number, startParam?: string) {
  clearSession();
  const initData = launchDataFor(userId, startParam ? { startParam } : {});
  setLaunchContext({ initData, platform: 'web', appVersion: 'integration' });
  return createSession(initData, 'web', 'integration');
}

describe('Интеграция мини-приложения с тремя микросервисами', () => {
  beforeAll(async () => {
    const response = await fetch(`${apiBase()}/catalog`);
    expect([200, 401]).toContain(response.status);
  });

  it('T-INT-01: выдаёт сессию по подписанным данным запуска MAX', async () => {
    const session = await loginAs(OWNER_ID);
    expect(session.token).toMatch(/^vvs_[A-Za-z0-9_-]{43}$/);
    expect(session.account.first_name).toBe(`Тест ${OWNER_ID}`);
    expect(session.start.kind).toBe('none');
    expect(Date.parse(session.expires_at)).toBeGreaterThan(Date.now());
    expect(getSession()?.token).toBe(session.token);
  });

  it('T-INT-02: отклоняет изменённые данные запуска', async () => {
    const broken = `${launchDataFor(OWNER_ID)}0`;
    await expect(createSession(broken, 'web', 'integration')).rejects.toMatchObject({
      name: 'ApiError',
      status: 401,
    });
    await loginAs(OWNER_ID);
  });

  it('T-INT-03: читает профиль и справочник', async () => {
    const [me, catalog] = await Promise.all([getMe(), getCatalog()]);
    expect(me.account.first_name).toBe(`Тест ${OWNER_ID}`);
    expect(me.limits.max_documents_per_organization).toBeGreaterThan(0);
    expect(['active', 'muted', 'stopped', 'unknown', 'unreachable', 'unavailable']).toContain(
      me.reminders_channel.state,
    );
    expect(catalog.business_categories.length).toBeGreaterThan(0);
    expect(catalog.regions.length).toBeGreaterThan(0);
    expect(catalog.document_types.length).toBeGreaterThan(0);
  });

  it('T-INT-04: создаёт организацию и повторяет запрос идемпотентно', async () => {
    const catalog = await getCatalog();
    const category = catalog.business_categories[0]!.code;
    const region = catalog.regions[0]!;
    const body = {
      id: ORG_ID,
      name: 'Интеграционное кафе',
      business_category_code: category,
      region_code: region.code,
      timezone: region.default_timezone,
      feature_codes: catalog.features.slice(0, 1).map((feature) => feature.code),
    };
    const created = await createOrganization(body);
    expect(created.id).toBe(ORG_ID);
    expect(created.my_role).toBe('owner');
    const repeated = await createOrganization(body);
    expect(repeated.id).toBe(ORG_ID);
    expect(repeated.version).toBe(created.version);

    const me = await getMe();
    expect(me.memberships.some((item) => item.organization_id === ORG_ID)).toBe(true);
  });

  it('T-INT-05: отдаёт подсказки типовых документов по профилю', async () => {
    const suggestions = await listSuggestions(ORG_ID);
    expect(Array.isArray(suggestions)).toBe(true);
    if (suggestions.length > 0) expect(suggestions[0]!.document_type_code).toBeTruthy();
  });

  it('T-INT-06: создаёт документ и показывает состояние плана напоминаний', async () => {
    const organization = await getOrganization(ORG_ID);
    const today = todayInTimeZone(organization.timezone);
    const document = await createDocument(ORG_ID, {
      id: DOC_ID,
      document_type_code: null,
      title: 'Лицензия интеграционная',
      number: 'ИНТ-1',
      issuer: null,
      responsible_label: 'Управляющий',
      notes: null,
      reference_url: 'https://example.test/doc',
      valid_from: today,
      valid_until: shiftDate(today, { days: 3 }),
      reminder_offsets_days: [3, 1, 0],
    });
    expect(document.status).toBe('expiring');
    expect(document.days_left).toBe(3);
    expect(['actual', 'pending', 'unavailable']).toContain(document.reminders_state);
    expect(document.reminder_offsets_days).toEqual([3, 1, 0]);
  });

  it('T-INT-07: план напоминаний становится актуальным (core → reminders)', async () => {
    let document = await getDocument(DOC_ID);
    for (let attempt = 0; attempt < 20 && document.reminders_state !== 'actual'; attempt += 1) {
      await new Promise((resolve) => setTimeout(resolve, 500));
      document = await getDocument(DOC_ID);
    }
    expect(document.reminders_state).toBe('actual');
    expect(document.next_reminder_at).toBeTruthy();
  });

  it('T-INT-08: реестр фильтруется, ищется и листается курсором', async () => {
    const all = await listDocuments(ORG_ID, { limit: 20 });
    expect(all.items.some((item) => item.id === DOC_ID)).toBe(true);

    const expiring = await listDocuments(ORG_ID, { status: 'expiring' });
    expect(expiring.items.every((item) => item.status === 'expiring')).toBe(true);

    const found = await listDocuments(ORG_ID, { q: 'интеграционн' });
    expect(found.items.some((item) => item.id === DOC_ID)).toBe(true);

    const nothing = await listDocuments(ORG_ID, { q: 'такого-документа-нет' });
    expect(nothing.items).toHaveLength(0);

    const firstPage = await listDocuments(ORG_ID, { limit: 1 });
    expect(firstPage.items).toHaveLength(1);
  });

  it('T-INT-09: пакетное создание документов при онбординге', async () => {
    const catalog = await getCatalog();
    const type = catalog.document_types[0]!;
    const items = await createDocumentsBatch(ORG_ID, [
      {
        id: uuidV4(),
        document_type_code: type.code,
        title: type.title,
        number: null,
        issuer: null,
        responsible_label: null,
        notes: null,
        reference_url: null,
        valid_from: null,
        valid_until: null,
        reminder_offsets_days: type.default_reminder_offsets_days,
      },
    ]);
    expect(items).toHaveLength(1);
    expect(items[0]!.status).toBe('no_expiry');
  });

  it('T-INT-10: изменение документа проверяет версию', async () => {
    const before = await getDocument(DOC_ID);
    const updated = await updateDocument(DOC_ID, {
      expected_version: before.version,
      title: 'Лицензия интеграционная (изменена)',
      responsible_label: 'Директор',
    });
    expect(updated.version).toBe(before.version + 1);
    expect(updated.title).toContain('изменена');

    await expect(
      updateDocument(DOC_ID, { expected_version: before.version, title: 'Ещё раз' }),
    ).rejects.toMatchObject({ name: 'ApiError', code: 'CONFLICT_VERSION' });
  });

  it('T-INT-11: продление создаёт новый период и историю', async () => {
    const before = await getDocument(DOC_ID);
    const start = shiftDate(before.current_period.valid_until ?? todayInTimeZone('Europe/Moscow'), { days: 1 });
    const renewed = await renewDocument(DOC_ID, {
      id: uuidV4(),
      valid_from: start,
      valid_until: shiftDate(start, { years: 1 }),
    });
    expect(renewed.current_period.valid_from).toBe(start);
    expect(renewed.periods.length).toBeGreaterThan(1);
    expect(renewed.status).toBe('valid');
  });

  it('T-INT-12: настройки напоминаний сохраняются', async () => {
    const before = await getNotificationSettings(ORG_ID);
    expect(before.enabled).toBe(true);
    const saved = await putNotificationSettings(ORG_ID, { enabled: true, local_time: '10:30' });
    expect(saved.local_time).toBe('10:30');
    const again = await getNotificationSettings(ORG_ID);
    expect(again.local_time).toBe('10:30');
  });

  it('T-INT-13: экспорт календаря отдаёт файл ICS по одноразовой ссылке', async () => {
    const link = await createCalendarExport(ORG_ID);
    expect(link.file_name).toMatch(/\.ics$/);
    const url = link.download_url.startsWith('http')
      ? link.download_url
      : `${apiBase().replace(/\/api\/v1$/, '')}${link.download_url}`;
    const response = await fetch(url);
    expect(response.status).toBe(200);
    const body = await response.text();
    expect(body).toContain('BEGIN:VCALENDAR');
    expect(body).toContain('BEGIN:VEVENT');
  });

  it('T-INT-14: приглашение создаётся, принимается вторым пользователем (core → bot)', async () => {
    const invite = await createInvite(ORG_ID, uuidV4(), 'editor');
    expect(invite.link_url).toContain('startapp=inv_');
    expect(invite.share_text.length).toBeGreaterThan(0);
    const token = new URL(invite.link_url).searchParams.get('startapp')!.replace(/^inv_/, '');

    const active = await listInvites(ORG_ID);
    expect(active.some((item) => item.id === invite.id)).toBe(true);

    const guestSession = await loginAs(GUEST_ID, `inv_${token}`);
    expect(guestSession.start.kind).toBe('invite');

    const preview = await previewInvite(token);
    expect(preview.organization_name).toBe('Интеграционное кафе');
    expect(preview.role).toBe('editor');

    const accepted = await acceptInvite(token);
    expect(accepted.organization_id).toBe(ORG_ID);

    const guestMe = await getMe();
    expect(guestMe.memberships.some((item) => item.organization_id === ORG_ID)).toBe(true);

    await loginAs(OWNER_ID);
    const members = await listMembers(ORG_ID);
    expect(members).toHaveLength(2);
    expect(members.some((member) => member.role === 'editor')).toBe(true);
  });

  it('T-INT-15: владелец организации получает сообщение о новом участнике', async () => {
    // Сообщение ставится в очередь core и доставляется рабочим bot-service,
    // поэтому ждём появления в буфере канала MAX.
    let joined = '';
    for (let attempt = 0; attempt < 20; attempt += 1) {
      const response = await fetch(`${botAdminBase()}/debug/stub/messages`);
      expect(response.status).toBe(200);
      const payload = (await response.json()) as { messages?: { text?: string }[] } | { text?: string }[];
      const messages = Array.isArray(payload) ? payload : (payload.messages ?? []);
      joined = messages.map((message) => message.text ?? '').join('\n');
      if (joined.includes('Интеграционное кафе')) break;
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
    expect(joined).toContain('Интеграционное кафе');
  });

  it('T-INT-16: наблюдатель не может изменить документ, роли меняются владельцем', async () => {
    const members = await listMembers(ORG_ID);
    const guest = members.find((member) => !member.is_me)!;
    await createInvite(ORG_ID, uuidV4(), 'viewer').catch(() => undefined);

    const { updateMemberRole } = await import('../../src/features/members/api');
    const updated = await updateMemberRole(ORG_ID, guest.account_id, 'viewer');
    expect(updated.role).toBe('viewer');

    await loginAs(GUEST_ID);
    await expect(
      updateDocument(DOC_ID, { expected_version: 99, title: 'Нельзя' }),
    ).rejects.toMatchObject({ name: 'ApiError', code: 'FORBIDDEN' });

    await loginAs(OWNER_ID);
  });

  it('T-INT-17: приглашение отзывается, участник исключается', async () => {
    const invite = await createInvite(ORG_ID, uuidV4(), 'viewer');
    await revokeInvite(invite.id);
    const active = await listInvites(ORG_ID);
    expect(active.some((item) => item.id === invite.id)).toBe(false);

    const members = await listMembers(ORG_ID);
    const guest = members.find((member) => !member.is_me)!;
    await removeMember(ORG_ID, guest.account_id);
    expect(await listMembers(ORG_ID)).toHaveLength(1);
  });

  it('T-INT-18: профиль организации изменяется с проверкой версии', async () => {
    const before = await getOrganization(ORG_ID);
    const updated = await updateOrganization(ORG_ID, {
      expected_version: before.version,
      name: 'Интеграционное кафе №2',
    });
    expect(updated.name).toBe('Интеграционное кафе №2');
    await expect(
      updateOrganization(ORG_ID, { expected_version: before.version, name: 'Ещё' }),
    ).rejects.toMatchObject({ code: 'CONFLICT_VERSION' });
  });

  it('T-INT-19: удаление документа снимает его с плана напоминаний', async () => {
    await deleteDocument(DOC_ID);
    const failure = await getDocument(DOC_ID).catch((error) => error);
    expect(failure).toBeInstanceOf(ApiError);
    expect((failure as ApiError).status).toBe(404);
    const list = await listDocuments(ORG_ID, { limit: 50 });
    expect(list.items.some((item) => item.id === DOC_ID)).toBe(false);
  });

  it('T-INT-20: недействительный токен получает 401, выход завершает сессию', async () => {
    await loginAs(OWNER_ID);
    const response = await fetch(`${apiBase()}/me`, { headers: { Authorization: `Bearer vvs_${'x'.repeat(43)}` } });
    expect(response.status).toBe(401);

    await deleteCurrentSession();
    expect(getSession()).toBeNull();

    setLaunchContext({ initData: 'broken-launch-data', platform: 'web', appVersion: 'integration' });
    await expect(getMe()).rejects.toMatchObject({ name: 'ApiError', status: 401 });
    await loginAs(OWNER_ID);
  });
});
