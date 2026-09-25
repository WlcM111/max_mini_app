# Карта реализованных экранов Mini App

Версия 1.0.0 · 24.09.2026. Нормативный источник — `frontend-architecture.md` §3, §4, §9.
Ниже зафиксировано фактически реализованное состояние.

## 1. Экраны

| Маршрут | Компонент | Данные | Действия | Роль | Состояния |
|---|---|---|---|---|---|
| `/` | `HomeRedirect` | `me` из контекста | — | — | мгновенный переход |
| `/welcome` | `WelcomePage` | — | «Продолжить» | — | статичный |
| `/onboarding/organization` | `OrganizationFormPage` | `GET /catalog` | черновик | — | загрузка, ошибка, ошибки полей |
| `/onboarding/features` | `FeaturesPage` | `GET /catalog` | `POST /organizations` | — | загрузка, ошибка, повтор при `CONFLICT_ID_REUSED` |
| `/onboarding/suggestions` | `SuggestionsPage` | `GET /suggestions` | выбор типов | owner | загрузка, пусто, ошибка |
| `/onboarding/dates` | `DatesPage` | `GET /catalog` | `POST /documents/batch` | owner | загрузка, ошибка, проверка дат |
| `/o/:orgId` | `DashboardPage` | `GET /organizations/{id}`, `GET /documents?limit=5`, `me` | переходы | viewer | загрузка, пусто, ошибка, баннер канала |
| `/o/:orgId/documents` | `DocumentsPage` | `GET /documents` (курсор) | фильтр, поиск, подгрузка | viewer | загрузка, пусто (фильтр/поиск), ошибка |
| `/o/:orgId/documents/new` | `DocumentFormPage` | `GET /catalog` | `POST /documents` | editor | отправка, ошибки полей, QR |
| `/d/:docId` | `DocumentCardPage` | `GET /documents/{id}`, `GET /organizations/{id}` | «Продлить», «Изменить», «Удалить» | viewer (действия — editor) | загрузка, 404, подтверждение удаления |
| `/d/:docId/edit` | `DocumentFormPage` | `GET /documents/{id}` | `PATCH /documents/{id}` | editor | конфликт версии, ошибки полей |
| `/d/:docId/renew` | `RenewPage` | `GET /documents/{id}` | `POST /renewals` | editor | загрузка, ошибка дат |
| `/o/:orgId/members` | `MembersPage` | `GET /members`, `GET /invites` | смена роли, исключение, выход, отзыв | viewer | загрузка, пусто, ошибка, подтверждение |
| `/o/:orgId/invite` | `InvitePage` | — | `POST /invites` | owner | создание, шеринг, копирование |
| `/o/:orgId/settings` | `SettingsPage` | `GET /notification-settings`, `GET /organizations/{id}`, `me` | `PUT` настроек, экспорт, удаление, выход | viewer | загрузка, ошибка, подтверждение |
| `/o/:orgId/settings/profile` | `OrganizationSettingsPage` | `GET /organizations/{id}`, `GET /catalog` | `PATCH /organizations/{id}` | editor | загрузка, конфликт версии |
| `/account` | `AccountPage` | `me` | `DELETE /sessions/current`, `DELETE /me` | — | подтверждение, ошибка |
| `/invite` | `InviteAcceptPage` | `POST /invites/preview` | `POST /invites/accept` | — | загрузка, недействительно, принято |
| экраны запуска | `NotInMaxPage`, `LaunchErrorPage` | — | «Повторить» | — | вне MAX, ошибка запуска, нет сети |
| `*` | `NotFoundPage` | — | «На главную» | — | — |

## 2. Переходы

```text
bootstrap ──┬─ нет initData ─────────────► NotInMax
            ├─ 401 / ошибка запуска ─────► LaunchError
            ├─ start=document ───────────► DocumentCard
            ├─ start=organization ───────► Dashboard
            ├─ start=invite ─────────────► InviteAccept ──принято──► Dashboard
            ├─ нет организаций ──────────► Welcome ─► OrganizationForm ─► Features
            │                                        ─► Suggestions ─► Dates ─► Dashboard
            └─ есть организации ─────────► Dashboard
Dashboard ─► Documents ─► DocumentCard ─► Edit | Renew
Dashboard ─► Members ─► Invite
Dashboard ─► Settings ─► OrganizationProfile | Account
```

Корневые экраны (`/`, `/welcome`, `/o/:orgId`, `/error/*`) — без системной кнопки «Назад».
На остальных `BackButton.show()`, обработчик — `navigate(-1)`; при открытии по диплинку
(история пуста) — переход на главный экран. После создания и удаления используется
`navigate(…, { replace: true })`.

## 3. Тексты состояний

| Ситуация | Текст |
|---|---|
| Пустой реестр | «Документов пока нет» + «Добавить документ», «Подобрать по профилю» |
| Пусто с фильтром | «Нет документов с таким статусом» |
| Пусто с поиском | «Ничего не найдено» |
| Нет участников | «Вы пока работаете один» |
| Все типы добавлены | «Все типовые документы уже добавлены» |
| Документ удалён | «Документ удалён или недоступен» + «К списку» |
| План пересчитывается | «Напоминания пересчитываются» |
| План недоступен | «План напоминаний временно недоступен» |
| Канал не активен | «Напоминания не приходят…» + «Открыть чат с ботом» |
| Статусы срока | «Просрочен», «Скоро истекает», «В порядке», «Бессрочный» + дни |
