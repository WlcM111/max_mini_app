# API-клиент мини-приложения и матрица интеграции

Версия 1.0.0 · 24.09.2026.

## 1. Устройство клиента

`src/api/client.ts` — единственное место сетевых вызовов приложения (кроме отправки
телеметрии на публичный `POST /client-events`). Построен на `openapi-fetch` поверх типов,
сгенерированных из `openapi.yaml` (`npm run gen-api`). Ручное описание типов API запрещено.

Возможности: типизированные запросы и ответы, заголовок `Authorization: Bearer` из
`sessionStore`, тайм-аут 10 с через `AbortController`, однократное обновление сессии при 401
с повтором исходного запроса, разбор `application/problem+json` в `ApiError`
(`status`, `code`, `detail`, `errors[]`, `request_id`, `Retry-After`), сетевые сбои →
`NetworkError`. Повторы GET (2 попытки: 0,5 с и 1,5 с) задаются политикой React Query,
изменяющие операции не повторяются автоматически.

## 2. Матрица операций

| Действие пользователя | Операция OpenAPI | Авторизация | Обновление состояния |
|---|---|---|---|
| Запуск | `POST /sessions` | нет (initData) | `sessionStore`, цель запуска |
| Выход | `DELETE /sessions/current` | Bearer | очистка хранилищ |
| Профиль | `GET /me` | Bearer | ключ `['me']` |
| Удаление аккаунта | `DELETE /me` | Bearer | перезапуск приложения |
| Справочник | `GET /catalog` | Bearer | ключ `['catalog']`, ETag |
| Создание организации | `POST /organizations` | Bearer | `['me']`, `['org', id]` |
| Дашборд | `GET /organizations/{id}` | Bearer | `['org', id]` |
| Профиль организации | `PATCH /organizations/{id}` | Bearer, editor | `['org']`, `['suggestions']`, `['me']` |
| Удаление организации | `DELETE /organizations/{id}` | Bearer, owner | полная инвалидация |
| Подсказки | `GET /organizations/{id}/suggestions` | Bearer, owner | `['suggestions', id]` |
| Реестр | `GET /organizations/{id}/documents` | Bearer | `['documents', id, filter]` |
| Создание документа | `POST /organizations/{id}/documents` | Bearer, editor | `['documents']`, `['org']`, `['suggestions']` |
| Пакет документов | `POST /organizations/{id}/documents/batch` | Bearer, editor | то же |
| Карточка | `GET /documents/{id}` | Bearer | `['document', id]` |
| Изменение | `PATCH /documents/{id}` | Bearer, editor | `['document']`, `['documents']`, `['org']` |
| Продление | `POST /documents/{id}/renewals` | Bearer, editor | то же |
| Удаление документа | `DELETE /documents/{id}` | Bearer, editor | удаление ключа, инвалидация списка |
| Участники | `GET /organizations/{id}/members` | Bearer | `['members', id]` |
| Смена роли | `PATCH …/members/{accountId}` | Bearer, owner | `['members', id]` |
| Исключение и выход | `DELETE …/members/{accountId}` | Bearer | `['members']`, `['me']` |
| Приглашения | `GET /organizations/{id}/invites` | Bearer, owner | `['invites', id]` |
| Создание приглашения | `POST /organizations/{id}/invites` | Bearer, owner | `['invites', id]` |
| Отзыв | `DELETE /invites/{inviteId}` | Bearer, owner | `['invites', id]` |
| Предпросмотр | `POST /invites/preview` | Bearer | — |
| Принятие | `POST /invites/accept` | Bearer | `['me']` |
| Настройки напоминаний | `GET/PUT …/notification-settings` | Bearer | `['notify', id]`, `['me']` |
| Экспорт календаря | `POST …/exports/calendar` | Bearer | одноразовая ссылка |
| Скачивание файла | `GET /downloads/{token}` | без Bearer | Bridge `downloadFile` |
| Телеметрия | `POST /client-events` | без Bearer | — |

## 3. Ответственность сервисов

Мини-приложение обращается только к публичному API core-service. Данные
reminders-service (`next_reminder_at`, `reminders_state`) и bot-service
(`reminders_channel.state`, `bot_chat_url`, ссылка приглашения) приходят через core —
прямых обращений браузера к внутренним gRPC-интерфейсам нет и не предусмотрено.

| Сервис | Что даёт интерфейсу | Как отображается |
|---|---|---|
| core-service | организации, документы, участники, приглашения, экспорт, сессии | все экраны |
| reminders-service | ближайшее напоминание, состояние плана | карточка и реестр: «напоминания пересчитываются», «план временно недоступен» |
| bot-service | состояние канала MAX, ссылка на чат, профиль бота для диплинка | баннер на дашборде и в настройках, кнопка «Открыть чат с ботом», ссылка приглашения |
