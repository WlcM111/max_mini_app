# Публичный HTTP API

Версия 1.0.0 · 20.09.2026. Нормативно — [openapi.yaml](../../openapi.yaml); здесь — сводка и правила поведения.

## 1. Операции

| operationId | Метод и путь | Роль | Успех | Возможные ошибки |
|---|---|---|---|---|
| createSession | POST /sessions | — | 201 | 400, 401 (LAUNCH_DATA_*), 429, 503 |
| deleteCurrentSession | DELETE /sessions/current | любой | 204 | 401 |
| getMe | GET /me | любой | 200 | 401, 429 |
| deleteMe | DELETE /me | любой | 204 | 401 |
| getCatalog | GET /catalog | любой | 200, 304 | 401 |
| createOrganization | POST /organizations | любой | 201, 200 (повтор) | 400, 401, 409 (CONFLICT_ID_REUSED, QUOTA_EXCEEDED), 429 |
| getOrganization | GET /organizations/{organizationId} | viewer | 200 | 401, 403, 404 |
| updateOrganization | PATCH /organizations/{organizationId} | editor | 200 | 400, 401, 403, 404, 409 (CONFLICT_VERSION) |
| deleteOrganization | DELETE /organizations/{organizationId} | owner | 204 | 401, 403, 404 |
| listSuggestions | GET /organizations/{organizationId}/suggestions | viewer | 200 | 401, 403, 404 |
| listDocuments | GET /organizations/{organizationId}/documents | viewer | 200 | 400, 401, 403, 404 |
| createDocument | POST /organizations/{organizationId}/documents | editor | 201, 200 | 400, 401, 403, 404, 409 |
| createDocumentsBatch | POST /organizations/{organizationId}/documents/batch | editor | 201 | 400, 401, 403, 404, 409 |
| getDocument | GET /documents/{documentId} | viewer | 200 | 401, 403, 404 |
| updateDocument | PATCH /documents/{documentId} | editor | 200 | 400, 401, 403, 404, 409 |
| deleteDocument | DELETE /documents/{documentId} | editor | 204 | 401, 403, 404 |
| renewDocument | POST /documents/{documentId}/renewals | editor | 201, 200 | 400, 401, 403, 404, 409 |
| listMembers | GET /organizations/{organizationId}/members | viewer | 200 | 401, 403, 404 |
| updateMemberRole | PATCH /organizations/{organizationId}/members/{accountId} | owner | 200 | 400, 401, 403, 404 |
| removeMember | DELETE /organizations/{organizationId}/members/{accountId} | owner; сам участник для себя | 204 | 401, 403, 404 |
| getMyNotificationSettings | GET /organizations/{organizationId}/notification-settings | viewer | 200 | 401, 403, 404 |
| putMyNotificationSettings | PUT /organizations/{organizationId}/notification-settings | viewer | 200 | 400, 401, 403, 404 |
| listInvites | GET /organizations/{organizationId}/invites | owner | 200 | 401, 403, 404 |
| createInvite | POST /organizations/{organizationId}/invites | owner | 201 | 400, 401, 403, 404, 409, 503 (DEPENDENCY_UNAVAILABLE) |
| revokeInvite | DELETE /invites/{inviteId} | owner | 204 | 401, 403, 404, 409 (INVITE_INVALID для принятого) |
| previewInvite | POST /invites/preview | любой | 200 | 400, 401, 404, 409, 429 |
| acceptInvite | POST /invites/accept | любой | 200 | 400, 401, 404, 409, 429 |
| createCalendarExport | POST /organizations/{organizationId}/exports/calendar | viewer | 201 | 401, 403, 404, 429 |
| downloadCalendar | GET /downloads/{downloadToken} | без токена | 200 `text/calendar` | 404, 410 (LINK_GONE) |
| postClientEvents | POST /client-events | без токена или любой | 202 | 400, 429 |

Не участник организации получает 404 (существование чужих ресурсов не раскрывается); участник с недостаточной ролью — 403.

## 2. Коды ошибок

| `code` | HTTP | Когда |
|---|---|---|
| VALIDATION_FAILED | 400 | нарушены ограничения схемы; неизвестные поля JSON; неверный курсор |
| UNAUTHENTICATED | 401 | нет заголовка, неверный формат, отозванный или истёкший токен |
| LAUNCH_DATA_INVALID | 401 | подпись, формат или будущий `auth_date` |
| LAUNCH_DATA_EXPIRED | 401 | `auth_date` старше 1 ч |
| FORBIDDEN | 403 | роль ниже требуемой; владелец пытается выйти или сменить свою роль |
| NOT_FOUND | 404 | ресурс отсутствует или пользователь не участник |
| INVITE_INVALID | 404, 409 | приглашение не найдено, отозвано, принято другим (404); отзыв принятого (409) |
| CONFLICT_VERSION | 409 | `expected_version` не совпал |
| CONFLICT_ID_REUSED | 409 | UUID уже использован другим пользователем, в другой организации или для приглашения |
| INVITE_EXPIRED | 409 | приглашение истекло |
| ALREADY_MEMBER | 409 | пользователь уже участник |
| QUOTA_EXCEEDED | 409 | превышены лимиты `Limits` или 50 активных приглашений |
| LINK_GONE | 410 | ссылка экспорта истекла или исчерпана |
| RATE_LIMITED | 429 | превышен лимит частоты; заголовок `Retry-After` |
| OVERLOADED | 503 | превышен предел одновременных запросов; `Retry-After: 1` |
| DEPENDENCY_UNAVAILABLE | 503 | bot не ответил, а операция без него невозможна |
| INTERNAL | 500 | непредвиденная ошибка; детали только в логах по `request_id` |

Поле `type` — `urn:vovremya:problem:<code в нижнем регистре через дефис>`, например `urn:vovremya:problem:conflict-version`. Ответ 413 при теле больше 256 KB формирует edge без problem+json.

## 3. Правила

| Тема | Правило |
|---|---|
| Идемпотентность создания | повтор с тем же `id` в той же организации от того же пользователя → 200 и текущее состояние; иначе 409 `CONFLICT_ID_REUSED`. Для периода продления — тот же документ. Для приглашения повтор всегда 409 (ссылка не восстанавливается, токен хранится хешем) |
| Оптимистичная блокировка | PATCH организации и документа требует `expected_version`; успешное изменение увеличивает `version` на 1 |
| Курсор | непрозрачная строка base64url от JSON `{"u": "YYYY-MM-DD" или "inf", "p": "<uuid>"}`; порядок `(coalesce(valid_until, infinity), public_id)`; неверный курсор → 400 |
| Сегодняшняя дата | вычисляется в часовом поясе организации на момент запроса |
| Кодировка | UTF-8; ответы сжимаются edge (gzip, zstd) |
| Заголовки | запрос: `Authorization`, `Content-Type: application/json`, `If-None-Match` (catalog); ответ: `X-Request-Id`, `ETag`, `Retry-After` |
| Кеширование | `GET /catalog` — `private, max-age=3600` и ETag; остальные — `no-store` |
