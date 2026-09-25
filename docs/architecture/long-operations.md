# Длительные операции и состояния

Версия 1.0.0 · 20.09.2026. Решение — ADR-010.

## D8-1. Напоминание в core (`core.reminders.status`)

```mermaid
stateDiagram-v2
  [*] --> planned: план построен при изменении документа, периода, отступов, участников, настроек
  planned --> handed_off: EnqueueNotification вернул QUEUED или duplicate
  planned --> planned: ошибка или таймаут gRPC, attempts плюс 1, next_attempt_at с backoff
  planned --> cancelled: продление, удаление отступа или документа, исключение участника, уведомления выключены
  planned --> skipped: now больше due_at плюс 24 ч
  handed_off --> [*]
  cancelled --> [*]
  skipped --> [*]
```

| Переход | Условие | Транзакция |
|---|---|---|
| → planned | `due_at ≥ now()`; участник с `notify_enabled`; период текущий с `valid_until` | та же, что изменение документа или настроек |
| planned → handed_off | ответ bot без ошибки | транзакция планировщика, строка заблокирована `FOR UPDATE SKIP LOCKED` |
| planned → planned | gRPC `UNAVAILABLE`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`; `next_attempt_at = now + min(5 мин, 10 с × 2^attempts) + случайно 0…5 с` | та же |
| planned → skipped | `now > due_at + 24 ч` (напоминание устарело) | та же |
| planned → cancelled | перепланирование | транзакция изменения |

Перепланирование (функция `domain.PlanReminders`) вычисляет желаемое множество `(period_id, account_id, days_before, due_at)` и сравнивает с существующими строками `planned`: недостающие вставляются (`ON CONFLICT DO NOTHING`), лишние переводятся в `cancelled`, строки `handed_off` не трогаются.

## D8-2. Сообщение в bot (`bot.outbound_messages.status`)

```mermaid
stateDiagram-v2
  [*] --> queued: EnqueueNotification или ответ на событие webhook
  queued --> sending: воркер взял lease на 60 с
  retry_wait --> sending: наступил next_attempt_at
  sending --> sent: MAX ответил 200
  sending --> retry_wait: 401, 429, 5xx, таймаут, сетевая ошибка
  sending --> failed: 400, 403, 404, 405 или исчерпаны 8 попыток
  sending --> retry_wait: lease истёк после сбоя процесса
  queued --> expired: now больше not_after
  retry_wait --> expired: now больше not_after
  sent --> [*]
  failed --> [*]
  expired --> [*]
```

## D8-3. Приглашение (`core.invites`)

```mermaid
stateDiagram-v2
  [*] --> active: POST invites
  active --> accepted: POST invites accept до expires_at
  active --> revoked: DELETE invites id
  active --> expired: now больше expires_at, вычисляется при чтении
  accepted --> [*]
  revoked --> [*]
  expired --> [*]
```

## D8-4. Получатель (`bot.recipients.state`)

```mermaid
stateDiagram-v2
  [*] --> unknown
  unknown --> active: bot_started или message_created
  active --> muted: dialog_muted
  muted --> active: dialog_unmuted
  active --> stopped: bot_stopped или dialog_removed
  muted --> stopped: bot_stopped или dialog_removed
  stopped --> active: bot_started
  active --> unreachable: MAX ответил 403 или 404 при отправке
  unreachable --> active: bot_started или message_created
```

## Отмена, повтор и конкурентность

| Ситуация | Поведение |
|---|---|
| Пользователь продлил документ, пока напоминание в `planned` | транзакция продления переводит напоминания старого периода в `cancelled` |
| Продление после `handed_off`, но до отправки | сообщение уйдёт (уже в очереди bot); текст содержит дату окончания, пользователь увидит в карточке новый срок. Отмена в bot не реализуется: окно ≤ 15 с + время очереди |
| Два экземпляра планировщика | `FOR UPDATE SKIP LOCKED` исключает двойную обработку |
| Рестарт core во время передачи | транзакция не зафиксирована → строка остаётся `planned` → повтор с тем же ключом → bot вернёт `duplicate=true` |
| Рестарт bot во время отправки | строка `sending` с истёкшим lease возвращается в `retry_wait`; возможен дубль, если MAX уже принял сообщение |
| Webhook повторён MAX | дедупликация по `sha256(тело)` |
