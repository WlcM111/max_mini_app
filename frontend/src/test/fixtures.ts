import type {
  Catalog,
  Document,
  DocumentListItem,
  DocumentPage,
  Me,
  Member,
  Organization,
  Session,
} from '../api/client';

// Фикстуры по схемам OpenAPI: используются в тестах компонентов и обработчиках MSW.

export const ORG_ID = '0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01';
export const DOC_ID = '2f8b6d41-9c3e-4a75-b1d8-6e5f4a3c2b10';
export const ACCOUNT_ID = '5d1f7c3a-9b2e-4c8a-8f61-3a7d2e5b9c04';

export const account = {
  id: ACCOUNT_ID,
  first_name: 'Михаил',
  last_name: null,
  username: null,
};

export const me: Me = {
  account,
  memberships: [
    { organization_id: ORG_ID, organization_name: 'Кафе «Пример»', role: 'owner', notify_enabled: true },
  ],
  reminders_channel: { state: 'active', bot_chat_url: 'https://max.ru/vovremya_local_bot' },
  limits: {
    max_organizations: 20,
    max_documents_per_organization: 500,
    max_members_per_organization: 30,
    max_reminder_offsets: 5,
  },
};

export const session: Session = {
  token: `vvs_${'a'.repeat(43)}`,
  expires_at: new Date(Date.now() + 12 * 3600 * 1000).toISOString(),
  account,
  start: { kind: 'none' },
};

export const organization: Organization = {
  id: ORG_ID,
  name: 'Кафе «Пример»',
  business_category_code: 'food_service',
  region_code: 'RU-SPE',
  timezone: 'Europe/Moscow',
  feature_codes: ['has_premises', 'sells_alcohol'],
  my_role: 'owner',
  version: 1,
  stats: { total: 3, expired: 1, expiring: 1, valid: 1, no_expiry: 0, next_valid_until: '2026-10-01' },
  created_at: '2026-09-01T09:00:00Z',
  updated_at: '2026-09-20T09:00:00Z',
};

export const catalog: Catalog = {
  version: 'abcdef0123456789',
  business_categories: [
    { code: 'food_service', title: 'Общественное питание' },
    { code: 'retail', title: 'Розничная торговля' },
  ],
  regions: [
    { code: 'RU-SPE', title: 'Санкт-Петербург', default_timezone: 'Europe/Moscow' },
    { code: 'RU-NVS', title: 'Новосибирская область', default_timezone: 'Asia/Novosibirsk' },
  ],
  features: [
    { code: 'has_premises', question: 'Есть помещение?', hint: 'Аренда или собственность' },
    { code: 'sells_alcohol', question: 'Продаёте алкоголь?', hint: null },
  ],
  document_types: [
    {
      code: 'alcohol_license',
      title: 'Лицензия на розничную продажу алкоголя',
      description: 'Разрешение на продажу алкогольной продукции',
      data_status: 'model',
      source: null,
      default_reminder_offsets_days: [60, 30, 7],
      renewal_steps: ['Собрать документы', 'Подать заявление'],
    },
  ],
};

export const documentItems: DocumentListItem[] = [
  {
    id: DOC_ID,
    organization_id: ORG_ID,
    document_type_code: 'alcohol_license',
    title: 'Лицензия на алкоголь',
    responsible_label: 'Управляющий',
    valid_until: '2026-10-01',
    status: 'expiring',
    days_left: 7,
    next_reminder_at: '2026-09-25T06:00:00Z',
    reminders_state: 'actual',
  },
  {
    id: '7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03',
    organization_id: ORG_ID,
    document_type_code: null,
    title: 'Договор аренды',
    responsible_label: null,
    valid_until: '2026-08-01',
    status: 'expired',
    days_left: -54,
    next_reminder_at: null,
    reminders_state: 'pending',
  },
];

export const documentPage: DocumentPage = { items: documentItems, next_cursor: null };

export const document: Document = {
  id: DOC_ID,
  organization_id: ORG_ID,
  document_type_code: 'alcohol_license',
  title: 'Лицензия на алкоголь',
  number: 'АЛ-123',
  issuer: 'Росалкогольрегулирование',
  responsible_label: 'Управляющий',
  notes: null,
  reference_url: 'https://example.test/license',
  current_period: {
    id: '4e9a7c30-5d1b-4f28-9e61-8c3a2f5d7b04',
    valid_from: '2025-10-01',
    valid_until: '2026-10-01',
    is_current: true,
    created_at: '2025-10-01T09:00:00Z',
  },
  periods: [
    {
      id: '4e9a7c30-5d1b-4f28-9e61-8c3a2f5d7b04',
      valid_from: '2025-10-01',
      valid_until: '2026-10-01',
      is_current: true,
      created_at: '2025-10-01T09:00:00Z',
    },
  ],
  status: 'expiring',
  days_left: 7,
  reminder_offsets_days: [60, 30, 7],
  next_reminder_at: '2026-09-25T06:00:00Z',
  reminders_state: 'actual',
  renewal_steps: ['Собрать документы', 'Подать заявление'],
  data_status: 'model',
  version: 1,
  can_edit: true,
  created_at: '2025-10-01T09:00:00Z',
  updated_at: '2026-09-20T09:00:00Z',
};

export const members: Member[] = [
  {
    account_id: ACCOUNT_ID,
    first_name: 'Михаил',
    last_name: null,
    role: 'owner',
    joined_at: '2026-09-01T09:00:00Z',
    is_me: true,
  },
  {
    account_id: '9d4f2a68-1c7b-4e39-85a0-7f2b6c4d1e08',
    first_name: 'Мария',
    last_name: 'Иванова',
    role: 'editor',
    joined_at: '2026-09-10T09:00:00Z',
    is_me: false,
  },
];
