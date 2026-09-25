# Отчёт реализации MAX Mini App (frontend)

Версия 4.0.0 · 24.09.2026 · этап 4. Предыдущие отчёты: `FIRST_SERVICE_REPORT.md` (reminders-service),
`SECOND_SERVICE_REPORT.md` (bot-service), `THIRD_SERVICE_REPORT.md` (core-service).

## 1. Что реализовано

Полностью реализовано клиентское приложение «Вовремя» — MAX Mini App: запуск из MAX и
идентификация, онбординг, реестр и карточка документов, формы создания, изменения и продления,
участники и приглашения, настройки напоминаний, экспорт календаря, аккаунт и удаление данных.

Объём: 62 файла TypeScript/TSX в `frontend/src` (без сгенерированной схемы API), 20 экранов,
13 файлов тестов, 4 файла браузерных сценариев. Главный бандл — 364 КБ, **116 КБ gzip**
(бюджет 200 КБ gzip, `frontend-architecture.md` §12).

## 2. Стек

Соответствует ADR-003 и `docs/frontend/frontend-architecture.md` §1: React 18.3.1, TypeScript 5.9
(`strict`, `noUncheckedIndexedAccess`), Vite 7, `@maxhub/max-ui` (точная версия без `^`),
`@tanstack/react-query` 5, `react-router` 7 (`createMemoryRouter`), `openapi-fetch` + типы
`openapi-typescript` из `openapi.yaml`, Vitest 4 + Testing Library + MSW 2, Playwright 1,
ESLint 9 + typescript-eslint.

## 3. Структура

Каталоги ровно по §2 нормативной архитектуры:

| Слой | Назначение |
|---|---|
| `src/app` | точка сборки: `bootstrap.ts`, `providers.tsx`, `router.tsx`, `routes.ts`, `ErrorBoundary.tsx` |
| `src/platform/max` | единственное место обращения к `window.WebApp`: Bridge, имитация, BackButton, share, download, сканер, haptics, ссылки |
| `src/api` | единственное место сетевых вызовов: клиент, ошибки, ключи кэша, сгенерированная схема |
| `src/session` | токен, аккаунт, цель запуска, роль в организации |
| `src/features/*` | экраны и их запросы: onboarding, organizations, documents, members, invites, settings, export, system |
| `src/shared` | переиспользуемые компоненты (`AppShell`, `StatusBadge`, `StateViews`, `DateField`, `OffsetChips`, `ConfirmDialog`, `ModelDataBadge`) и утилиты (даты, склонения, UUID, валидация, телеметрия, хранилища) |

Правила зависимостей соблюдены: `features/*` не импортируют друг друга напрямую (кроме публичных
компонентов `shared` и вызова API соседней области через её `api.ts`), `fetch` вызывается только
в `api/client.ts` и `shared/lib/telemetry.ts` (телеметрия — отдельный публичный endpoint),
`window.WebApp` — только в `platform/max`. Запрет `any` и `dangerouslySetInnerHTML` проверяется
линтером (`eslint.config.js`).

## 4. Реализованные экраны

Все 20 маршрутов карты экранов (§3 архитектуры): `/`, `/welcome`, `/onboarding/organization`,
`/onboarding/features`, `/onboarding/suggestions`, `/onboarding/dates`, `/o/:orgId`,
`/o/:orgId/documents`, `/o/:orgId/documents/new`, `/o/:orgId/members`, `/o/:orgId/invite`,
`/o/:orgId/settings`, `/o/:orgId/settings/profile`, `/d/:docId`, `/d/:docId/edit`,
`/d/:docId/renew`, `/account`, `/invite`, `/error/*` (экраны запуска), `*`.

Подробная карта с состояниями и переходами — `docs/frontend/screens-map.md`.

## 5. Пользовательские сценарии

| Сценарий | Реализация | Подтверждение |
|---|---|---|
| Запуск из чата бота, идентификация | `app/bootstrap.ts` → `POST /sessions` с `initData` | T-INT-01, T-INT-02 |
| Онбординг: профиль, признаки, подсказки, сроки | 5 экранов `features/onboarding` | T-INT-04, T-INT-05, T-INT-09 |
| Реестр: фильтры, поиск, подгрузка | `DocumentsPage` + `useInfiniteQuery` | T-INT-08, T-FE-PAGE |
| Добавление документа | `DocumentFormPage` (клиентский UUID) | T-INT-06, T-FE-DUP |
| Карточка: статус, напоминание, шаги продления | `DocumentCardPage` | T-INT-06, T-INT-07 |
| Изменение с проверкой версии | `PATCH` с `expected_version` | T-INT-10 |
| Продление | `RenewPage` → `POST /renewals` | T-INT-11 |
| Удаление | подтверждение + `DELETE` | T-INT-19 |
| Участники и роли | `MembersPage` | T-INT-16, T-INT-17 |
| Приглашение и приём по диплинку | `InvitePage`, `InviteAcceptPage` | T-INT-14 |
| Настройки напоминаний | `SettingsPage` → `PUT /notification-settings` | T-INT-12 |
| Экспорт календаря | `CalendarExportButton` (два шага) | T-INT-13 |
| Аккаунт: выход и удаление данных | `AccountPage` | T-INT-20 |

## 6. Интеграция с MAX

Реализована через адаптер `platform/max` строго по подтверждённым фактам платформы
(`docs/max/max-platform-facts.md`) и таблице `docs/max/max-integration-spec.md` §6.
Реестр использованных механизмов — `docs/frontend/max-api-registry.md`.

Имитация окружения (`mockBridge.ts`) подключается динамическим `import()` только при
`VITE_MOCK_BRIDGE === 'true'`; в prod-сборке её нет, сборка образа падает, если строка
`devonly` попала в бандл (`deploy/edge/Dockerfile`).

## 7. Авторизация и сессия

`POST /sessions` с `init_data`, `platform`, `app_version`; токен `vvs_<43 символа>` хранится
в памяти и `sessionStorage['vovremya.session']` вместе с `expires_at`; заголовок
`Authorization: Bearer`; cookie не используются. При 401 клиент однократно обновляет сессию
по сохранённой строке запуска и повторяет исходный запрос (single-flight, повторные параллельные
401 ждут одного обновления); повторный 401 — экран перезапуска. Выход — `DELETE /sessions/current`
с очисткой хранилищ. Строка `initData` не логируется и не попадает в телеметрию.

## 8. Управление состоянием

Серверные данные — React Query по ключам §6 архитектуры (`me`, `catalog`, `org`, `documents`,
`document`, `members`, `invites`, `notify`, `suggestions`), `staleTime` 30–60 с, справочник —
бесконечно, повторы GET дважды (0,5 с и 1,5 с) только для сети, 429 и 5xx; изменения не
повторяются автоматически. Оптимистичных обновлений нет: источник статусов — сервер.
Глобально хранится только сессия и последняя организация (`localStorage['vovremya.lastOrg']`);
черновик онбординга — `sessionStorage['vovremya.onboarding']`; состояние форм — локальное.

## 9. Ошибки, загрузка, пустые состояния

Таблица кодов §10 архитектуры реализована в `api/errors.ts` (`messageForError`): `FORBIDDEN`,
`NOT_FOUND`, `CONFLICT_VERSION`, `CONFLICT_ID_REUSED`, `QUOTA_EXCEEDED`, `INVITE_*`,
`RATE_LIMITED`/`OVERLOADED` с `Retry-After`, `DEPENDENCY_UNAVAILABLE`, `LAUNCH_DATA_*`.
Ошибки полей формы (`problem.errors[].field`) показываются у соответствующих полей.
Для каждого экрана реализованы загрузка (скелетоны), пустое состояние и ошибка с «Повторить»
(`shared/ui/StateViews.tsx`). Непойманные ошибки рендера перехватывает `ErrorBoundary`.

## 10. Защита от повторных действий

Идентификатор создаваемой сущности (документ, период продления, организация, приглашение)
генерируется один раз при открытии экрана и переиспользуется при повторной отправке —
сервер отвечает идемпотентно. Дополнительно кнопка блокируется на время запроса, действует
синхронный замок от двойного клика и запрет повторной отправки после успеха
(`T-FE-DUP` проверяет, что двойное нажатие создаёт один запрос, а повтор после сетевой
ошибки отправляет тот же UUID).

## 11. Мобильная адаптация и тема

Одна колонка 320–640 px, `100dvh`, `env(safe-area-inset-*)`, закреплённая нижняя панель действий,
зоны нажатия ≥ 44 px, горизонтальная прокрутка только внутри ленты фильтров. Тема: `MaxUI`
с `colorScheme` из `prefers-color-scheme` (с подпиской на изменение) и `platform` из Bridge;
собственные цвета — только для статусов и всегда с текстом.

## 12. Доступность

Подписи у всех полей (`label + htmlFor`), ошибки с `role="alert"` и `aria-live="polite"`,
фокус переводится на заголовок при смене экрана (`AppShell`), фильтры — `aria-pressed`,
диалоги — `role="dialog"` с `aria-modal` и фокусом на главной кнопке, статусы читаются текстом.

## 13. Принятые инженерные решения

| Решение | Основание |
|---|---|
| `@maxhub/max-ui@0.2.0` (точная версия) | последняя 0.x, совместимая с React 18.3.1; см. противоречие C-FE-01 |
| Позднее связывание `fetch` в `openapi-fetch` | клиент создаётся при загрузке модуля; без позднего связывания сетевой слой невозможно подменить в тестах |
| Телеметрия отправляется отдельным `fetch` | `POST /client-events` не требует авторизации; так исключена цикличность зависимостей `api → session → api` |
| Интеграционные тесты выполняют код мини-приложения в Node | тот же API-клиент и те же функции экранов проверяются против настоящих сервисов без браузера |
| Тип документа не передаётся в `PATCH` | `DocumentUpdate` в OpenAPI 1.1.0 не содержит `document_type_code` |

## 14. Обнаруженные противоречия и дефекты

| ID | Что обнаружено | Разрешение |
|---|---|---|
| **C-FE-01** | `docs/frontend/frontend-architecture.md` §1 требует «последнюю 0.x» `@maxhub/max-ui`, но версии ≥ 0.2.1 (включая текущую 0.5.0) объявляют peer-зависимость ровно `react@19.2.8`, тогда как ADR-003 и handoff §27 фиксируют React 18 как неизменяемое решение | зафиксирована `@maxhub/max-ui@0.2.0` — последняя 0.x с peer `react@^18.3.1`. Путь обновления: перевод на React 19 + max-ui 0.5.0 отдельным решением (нужен пересмотр ADR-003) |
| **D-14** | образ `edge` не собирался: `COPY frontend/…` при отсутствующем каталоге (дефект этапа 3, устранён появлением реального каталога) | `deploy/edge/Dockerfile` вновь собирает интерфейс; заглушка удалена, в compose возвращены `VITE_MOCK_BRIDGE` и `VITE_APP_VERSION` |
| **D-15** | ограничители частоты core (10 rps на аккаунт) прерывали пакет автоматических проверок | на локальном стенде `scripts/run_local_stack.sh` пределы подняты переменными окружения; значения по умолчанию в compose и на стенде MAX не изменены |

Дефектов backend, требующих исправления кода, на этом этапе не обнаружено: все 20 интеграционных
сценариев прошли против неизменённых сервисов.

## 15. Проверки

`npm run lint`, `npm run typecheck`, `npm test -- --run` (63 теста), `npm run build` (116 КБ gzip),
`npm run test:integration` против настоящих трёх сервисов (20 сценариев), регрессия backend
(17 пакетов Go, `-race`, `bash test/e2e/run.sh`). Подробности — `docs/testing/frontend-test-report.md`.
