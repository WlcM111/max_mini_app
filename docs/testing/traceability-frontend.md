# Матрица трассируемости требований Mini App

Версия 1.0.0 · 24.09.2026. Источники: `docs/requirements/requirements-registry.md`,
`docs/handoffs/frontend-miniapp.md` §3, `docs/frontend/frontend-architecture.md`,
`openapi.yaml` 1.1.0. Тесты: `frontend/src/**/*.test.ts(x)` (T-FE-*),
`frontend/test/integration/api.int.test.ts` (T-INT-*), `frontend/e2e/*.spec.ts` (T-E2E-*).

| ID | Требование | Экран / модуль | Операция API | Тест | Статус |
|---|---|---|---|---|---|
| FR-01 | Идентификация по данным запуска MAX | `bootstrap.ts`, `platform/max` | `POST /sessions` | T-INT-01, T-INT-02, T-FE-MAX | выполнено |
| FR-02 | Сессия и выход | `session/*`, `AccountPage` | `POST /sessions`, `DELETE /sessions/current` | T-FE-API, T-INT-20 | выполнено |
| FR-03 | Справочник | `OrganizationFormPage`, `DocumentFormPage` | `GET /catalog` | T-INT-03 | выполнено |
| FR-04 | Организация: создание и профиль | `FeaturesPage`, `OrganizationSettingsPage` | `POST /organizations`, `PATCH` | T-INT-04, T-INT-18 | выполнено |
| FR-05 | Подсказки типовых документов | `SuggestionsPage` | `GET /suggestions` | T-INT-05 | выполнено |
| FR-06 | Реестр с фильтрами и поиском | `DocumentsPage` | `GET /documents` | T-FE-PAGE, T-INT-08 | выполнено |
| FR-07 | Карточка, история, шаги продления | `DocumentCardPage` | `GET /documents/{id}` | T-FE-PAGE, T-INT-11 | выполнено |
| FR-20 | Чек-лист подготовки к продлению | `DocumentCardPage`, `renewalChecklist.ts` | данные шагов из `GET /documents/{id}` | unit-тесты `renewalChecklist`, T-FE-PAGE (чек-лист) | выполнено |
| FR-08 | Добавление, в том числе пакетом | `DocumentFormPage`, `DatesPage` | `POST /documents`, `/batch` | T-FE-DUP, T-INT-06, T-INT-09 | выполнено |
| FR-09 | Продление | `RenewPage` | `POST /renewals` | T-INT-11 | выполнено |
| FR-10 | Участники, роли, приглашения | `MembersPage`, `InvitePage`, `InviteAcceptPage` | члены и приглашения | T-FE-PAGE, T-INT-14, T-INT-16, T-INT-17 | выполнено |
| FR-11 | Настройки напоминаний участника | `SettingsPage` | `GET/PUT /notification-settings` | T-INT-12 | выполнено |
| FR-13 | Показ ближайшего напоминания | `DocumentCardPage`, `DocumentsPage` | `reminders_state`, `next_reminder_at` | T-FE-PAGE, T-INT-07 | выполнено |
| FR-14 | Экспорт в календарь | `CalendarExportButton` | `POST /exports/calendar` | T-INT-13 | выполнено |
| FR-15 | Удаление аккаунта | `AccountPage` | `DELETE /me` | реализовано, проверено вручную по API | выполнено |
| FR-16 | Сообщение владельцу о новом участнике | цепочка core → bot | `POST /invites/accept` | T-INT-15 | выполнено |
| FR-18 | Демонстрационные данные | `seed-demo` backend + интерфейс | — | README §12 | выполнено |
| BR-02 | Квоты продукта | сообщения об ошибке `QUOTA_EXCEEDED` | все создающие операции | `messageForError` | выполнено |
| BR-04 | Пометка модельных данных | `ModelDataBadge` | `data_status` | T-FE-PAGE (карточка) | выполнено |
| BR-05 | Мини-приложение — основной интерфейс | всё приложение | — | T-E2E-01…06 (BLOCKED, см. отчёт) | выполнено, браузерная проверка заблокирована |
| NFR-01 | Первый экран ≤ 3 с | ленивые чанки, бандл 116 КБ gzip | — | `npm run build` | выполнено |
| NFR-03 | Мобильная адаптация | `app.css`, `AppShell` | — | проверка вёрстки, T-FE-A11Y | выполнено |
| NFR-05 | Идемпотентность повторов | клиентские UUID | создающие операции | T-FE-DUP, T-INT-04 | выполнено |
| NFR-06 | Уведомление об обработке данных и удаление | `WelcomePage`, `AccountPage` | `DELETE /me` | реализовано | выполнено |
| NFR-09 | Понятные ошибки и восстановление | `StateViews`, `messageForError` | все | T-FE-STATE, T-FE-API | выполнено |
| PLAT-07 | Диплинки напоминаний и приглашений | `chooseInitialPath` | `session.start` | T-FE-NAV, T-INT-14 | выполнено |
| PLAT-08 | Работа в клиентах MAX (iOS, Android, desktop, web) | `platform/max`, тема | — | требует стенда MAX (MAX-04) | BLOCKED |
