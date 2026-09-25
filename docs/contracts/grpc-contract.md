# gRPC-контракт core → bot

Версия 1.0.0 · 20.09.2026. Нормативно — [messaging.proto](../../api/proto/vovremya/bot/v1/messaging.proto). Транспорт: h2c внутри сети compose, адрес `bot:9090`; сервер также реализует `grpc.health.v1.Health`; reflection включён только при `APP_ENV=local`.

## 1. RPC

| RPC | Назначение | Валидация на сервере | Ответ | Ошибки | Deadline клиента |
|---|---|---|---|---|---|
| EnqueueNotification | поставить сообщение в очередь | ключ `^[a-z0-9:_-]{8,200}$`; `kind` ≠ UNSPECIFIED; `recipient_max_user_id` > 0; текст 1–4000; кнопок 0–3; текст кнопки 1–64; payload `^[A-Za-z0-9_-]{0,512}$` или url `https://` ≤ 2048; `not_after` в (now; now + 7 сут] | `notification_id`, `status`, `duplicate` | INVALID_ARGUMENT, ALREADY_EXISTS (ключ с другим содержимым), RESOURCE_EXHAUSTED (очередь > 50 000), UNAVAILABLE (БД) | 2 с |
| GetNotificationStatus | состояние доставки | ключ по тому же шаблону | `status`, `attempts`, `sent_at`, `last_error_code` | NOT_FOUND, INVALID_ARGUMENT, UNAVAILABLE | 2 с |
| GetRecipientStatus | можно ли писать пользователю | `max_user_id` > 0 | `state`, `state_changed_at`, `bot_chat_url` | INVALID_ARGUMENT, UNAVAILABLE | 2 с |
| GetBotProfile | ник бота и шаблон диплинка | — | `username`, `display_name`, `chat_url`, `open_app_link_template`, `mode` | UNAVAILABLE (профиль ещё не загружен из MAX) | 2 с |

`request_hash` для идемпотентности — SHA-256 от детерминированной сериализации (`proto.MarshalOptions{Deterministic: true}`) запроса с очищенным полем `idempotency_key`.

## 2. Поведение клиента (core)

| Код | Действие core |
|---|---|
| OK | напоминание → `handed_off`, сохраняется `bot_notification_id` |
| UNAVAILABLE, DEADLINE_EXCEEDED, RESOURCE_EXHAUSTED | повтор по backoff планировщика; для `/me` — `state = unavailable`; для приглашения — 503 `DEPENDENCY_UNAVAILABLE`, если нет кэша профиля |
| INVALID_ARGUMENT, ALREADY_EXISTS | ошибка программы: напоминание → `skipped`, `last_error_code = grpc_invalid`, лог error |
| NOT_FOUND | только для GetNotificationStatus: считать сообщение не поставленным |

Клиент использует один `grpc.ClientConn` на процесс, `keepalive` 30 с, без автоматических повторов на уровне gRPC (повторы — логика планировщика).

## 3. Соответствие сообщений и таблиц bot

| Поле proto | Столбец |
|---|---|
| `idempotency_key` | `outbound_messages.idempotency_key` |
| `kind` REMINDER / MEMBER_JOINED | `kind` `reminder` / `member_joined` |
| `recipient_max_user_id` | `recipient_max_user_id` |
| `text`, `silent`, `not_after` | `text`, `silent`, `not_after` |
| `buttons[i]` | `outbound_buttons(position = i + 1, text, action, open_app_payload, url)` |
| `notification_id` | `public_id` |
| NotificationStatus QUEUED…EXPIRED | `status` `queued`, `sending`, `sent`, `retry_wait`, `failed`, `expired` |
| RecipientState UNKNOWN…UNREACHABLE | `recipients.state`; отсутствие строки → UNKNOWN |
