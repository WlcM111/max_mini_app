// Нагрузочный сценарий профиля P3 (целевой): 150 RPS смешанной нагрузки на core через edge.
// Сессии создаются настоящим POST /sessions с initData, подписанным dev-токеном (только local).
import http from 'k6/http';
import crypto from 'k6/crypto';
import { check } from 'k6';

const BASE = __ENV.K6_BASE_URL || 'http://localhost:8080/api/v1';
const TOKEN = __ENV.K6_BOT_TOKEN || 'devonly-local-bot-token';

export const options = {
  scenarios: {
    mixed: {
      executor: 'ramping-arrival-rate',
      startRate: 10, timeUnit: '1s', preAllocatedVUs: 200, maxVUs: 400,
      stages: [
        { target: 150, duration: '2m' },
        { target: 150, duration: '5m' },
        { target: 300, duration: '1m' },
        { target: 0, duration: '30s' },
      ],
    },
  },
  thresholds: {
    'http_req_failed{expected:yes}': ['rate<0.01'],
    'http_req_duration{phase:steady}': ['p(95)<300', 'p(99)<800'],
  },
};

function initData(userId) {
  const user = JSON.stringify({ id: userId, first_name: 'Нагрузка', last_name: null, username: null, language_code: 'ru', photo_url: null });
  const f = { auth_date: String(Math.floor(Date.now() / 1000)), chat: JSON.stringify({ id: userId, type: 'DIALOG' }), query_id: `q-${userId}-${Date.now()}`, user };
  const secret = crypto.hmac('sha256', 'WebAppData', TOKEN, 'binary');
  const launch = Object.keys(f).sort().map((k) => `${k}=${f[k]}`).join('\n');
  f.hash = crypto.hmac('sha256', secret, launch, 'hex');
  return Object.keys(f).map((k) => `${k}=${encodeURIComponent(f[k])}`).join('&');
}

function uuid4() {
  const b = new Uint8Array(crypto.randomBytes(16));
  b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80;
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

const state = {};

function bootstrap() {
  const userId = 200000 + __VU;
  const s = http.post(`${BASE}/sessions`, JSON.stringify({ init_data: initData(userId), platform: 'web' }), { headers: { 'Content-Type': 'application/json' }, tags: { phase: 'bootstrap', expected: 'yes' } });
  check(s, { 'session 201': (r) => r.status === 201 });
  state.h = { headers: { Authorization: `Bearer ${s.json('token')}`, 'Content-Type': 'application/json' } };
  const orgId = uuid4();
  http.post(`${BASE}/organizations`, JSON.stringify({ id: orgId, name: `Нагрузка ${__VU}`, business_category_code: 'food_service', region_code: 'RU-SPE', timezone: 'Europe/Moscow', feature_codes: ['has_premises', 'has_employees'] }), state.h);
  const items = [];
  for (let i = 0; i < 15; i++) {
    const d = new Date(Date.now() + (i * 20 - 30) * 86400000).toISOString().slice(0, 10);
    items.push({ id: uuid4(), title: `Документ ${i}`, valid_until: d, reminder_offsets_days: [30, 7, 1] });
  }
  const b = http.post(`${BASE}/organizations/${orgId}/documents/batch`, JSON.stringify({ items }), state.h);
  state.org = orgId;
  state.docs = b.status === 201 ? b.json('items').map((x) => x.id) : [];
}

export default function () {
  if (!state.h) { bootstrap(); return; }
  const tags = { tags: { phase: 'steady', expected: 'yes' } };
  const h = Object.assign({}, state.h, tags);
  const r = Math.random();
  if (r < 0.25) http.get(`${BASE}/organizations/${state.org}/documents?limit=50`, h);
  else if (r < 0.45) http.get(`${BASE}/organizations/${state.org}`, h);
  else if (r < 0.60) http.get(`${BASE}/me`, h);
  else if (r < 0.77) http.get(`${BASE}/documents/${state.docs[Math.floor(Math.random() * state.docs.length)]}`, h);
  else if (r < 0.85) http.get(`${BASE}/catalog`, h);
  else if (r < 0.93) http.post(`${BASE}/organizations/${state.org}/documents`, JSON.stringify({ id: uuid4(), title: 'Новый', valid_until: '2027-03-01' }), h);
  else {
    const id = state.docs[Math.floor(Math.random() * state.docs.length)];
    const d = http.get(`${BASE}/documents/${id}`, h);
    if (d.status === 200) http.post(`${BASE}/documents/${id}/renewals`, JSON.stringify({ id: uuid4(), valid_from: '2026-10-01', valid_until: '2027-10-01' }), h);
  }
}
