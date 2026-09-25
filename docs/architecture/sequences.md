# Последовательности сценариев

Версия 1.0.0 · 20.09.2026. Коды ошибок — `ErrorCode` из [openapi.yaml](../../openapi.yaml).

## D7. Авторизация и инициализация

```mermaid
sequenceDiagram
  autonumber
  actor U as Пользователь
  participant MC as Клиент MAX
  participant FE as Mini App
  participant BR as MAX Bridge
  participant E as edge
  participant C as core
  participant DB as PostgreSQL
  participant B as bot
  U->>MC: нажимает кнопку бота или диплинк
  MC->>FE: открывает https://домен/ с фрагментом WebAppData
  FE->>BR: читает window.WebApp.initData и platform
  alt initData отсутствует
    FE-->>U: экран Откройте приложение из чата с ботом
  else initData есть
    FE->>E: POST /api/v1/sessions init_data, platform
    E->>C: прокси, X-Request-Id
    C->>C: проверка HMAC, единственность ключей, срок auth_date
    alt подпись неверна или данные старше 1 ч
      C-->>FE: 401 LAUNCH_DATA_INVALID или LAUNCH_DATA_EXPIRED
      FE-->>U: экран Перезапустите приложение
    else подпись верна
      C->>DB: upsert accounts по max_user_id, insert sessions с token_hash
      C->>DB: insert audit_events session.created
      C-->>FE: 201 token, expires_at, account, start
      FE->>FE: токен в память и sessionStorage
      par параллельно
        FE->>C: GET /api/v1/me с Bearer
        C->>DB: memberships, organizations
        C->>B: GetRecipientStatus, deadline 2 с
        B-->>C: state, bot_chat_url
        C-->>FE: 200 Me
      and
        FE->>C: GET /api/v1/catalog, If-None-Match
        C-->>FE: 200 Catalog или 304
      end
      FE-->>U: экран цели start, онбординг или дашборд
    end
  end
  Note over FE,C: При 401 UNAUTHENTICATED в любом запросе FE один раз повторяет POST /sessions с тем же initData
```

## D6-1. Онбординг и пакетное создание документов

```mermaid
sequenceDiagram
  autonumber
  actor U as Владелец
  participant FE as Mini App
  participant C as core
  participant DB as PostgreSQL
  U->>FE: название, вид деятельности, регион, признаки
  FE->>C: POST /organizations id=клиентский UUID
  C->>DB: BEGIN, проверка квоты 20 организаций
  C->>DB: insert organizations, organization_features, memberships role owner
  C->>DB: insert audit_events, COMMIT
  C-->>FE: 201 Organization
  FE->>C: GET /organizations/id/suggestions
  C->>DB: правила применимости минус добавленные типы
  C-->>FE: 200 SuggestionList
  U->>FE: отмечает документы и вводит даты
  FE->>C: POST /organizations/id/documents/batch до 30 элементов
  C->>DB: BEGIN, SELECT organizations FOR UPDATE, проверка квоты 500
  loop каждый документ
    C->>DB: insert documents, document_periods is_current, document_reminder_offsets
    C->>DB: insert reminders planned для каждого участника с notify_enabled
  end
  C->>DB: COMMIT
  C-->>FE: 201 DocumentsBatchResult
  FE-->>U: дашборд со сводкой просрочено, скоро, в порядке
```

## D6-2. Напоминание и открытие карточки

```mermaid
sequenceDiagram
  autonumber
  participant S as core ReminderScheduler
  participant DB as PostgreSQL core
  participant B as bot
  participant BDB as PostgreSQL bot
  participant W as bot DeliveryWorker
  participant M as MAX Bot API
  actor U as Участник
  participant FE as Mini App
  loop каждые 15 с
    S->>DB: BEGIN, SELECT reminders planned FOR UPDATE SKIP LOCKED LIMIT 200
    S->>B: EnqueueNotification key rem период участник дни, not_after
    B->>BDB: insert outbound_messages ON CONFLICT idempotency_key
    B-->>S: notification_id, status QUEUED, duplicate
    S->>DB: UPDATE reminders handed_off, COMMIT
  end
  W->>BDB: UPDATE status sending, locked_until, SKIP LOCKED
  W->>M: POST /messages?user_id=..., текст и кнопка link на диплинк doc_uuid
  alt 200
    M-->>W: message
    W->>BDB: status sent, sent_at
  else 429, 5xx, таймаут
    W->>BDB: status retry_wait, next_attempt_at с backoff
  end
  M-->>U: сообщение в чате с ботом
  U->>FE: нажимает Открыть документ, start_param doc_uuid
  FE->>FE: bootstrap по D7, start kind document
  FE->>DB: через core GET /documents/uuid с проверкой роли
  FE-->>U: карточка документа с кнопкой Продлить
```

## D6-3. Приглашение сотрудника

```mermaid
sequenceDiagram
  autonumber
  actor O as Владелец
  participant FE as Mini App владельца
  participant C as core
  participant B as bot
  participant DB as PostgreSQL
  actor N as Сотрудник
  participant FE2 as Mini App сотрудника
  O->>FE: Пригласить, роль editor
  FE->>C: POST /organizations/id/invites id, role
  C->>B: GetBotProfile, кэш 10 минут
  B-->>C: username, open_app_link_template
  C->>DB: insert invites token_hash, expires_at now плюс 72 ч
  C-->>FE: 201 link_url https://max.ru/бот?startapp=inv_токен, share_text
  O->>FE: Отправить в MAX
  FE->>FE: WebApp.shareMaxContent text и link в обработчике клика
  N->>FE2: открывает ссылку в MAX
  FE2->>C: POST /sessions, start kind invite
  FE2->>C: POST /invites/preview token
  C-->>FE2: organization_name, role, inviter_first_name
  N->>FE2: Принять
  FE2->>C: POST /invites/accept token
  C->>DB: BEGIN, SELECT invites FOR UPDATE, проверки срока и отзыва, квота 30
  C->>DB: insert memberships, update invites accepted, reminders planned для нового участника
  C->>DB: COMMIT
  C->>B: EnqueueNotification member_joined владельцу
  C-->>FE2: 200 organization_id, role
  FE2-->>N: дашборд организации
```

## D6-4. Конкурентное изменение документа

```mermaid
sequenceDiagram
  autonumber
  participant A as Редактор 1
  participant X as Редактор 2
  participant C as core
  participant DB as PostgreSQL
  A->>C: PATCH /documents/id expected_version 3
  X->>C: PATCH /documents/id expected_version 3
  C->>DB: UPDATE documents SET version 4 WHERE version 3 для запроса A
  C-->>A: 200 Document version 4
  C->>DB: UPDATE documents WHERE version 3 для запроса X, 0 строк
  C-->>X: 409 CONFLICT_VERSION
  X->>C: GET /documents/id
  C-->>X: 200 version 4, пользователь видит изменения и повторяет правку
```
