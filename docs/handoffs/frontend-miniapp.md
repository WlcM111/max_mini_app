# Задание на реализацию frontend Mini App

Версия 1.0.0 · 20.09.2026 · исполнитель: Разработчик A · reviewer: Разработчик B. Комплект — `handoff-frontend.zip` (состав — [README](README.md)). Мини-приложение — главный результат проекта: через него проходят все пользовательские сценарии.

## 0. Состояние на 24.09.2026 (готовность backend)

Все три backend-микросервиса реализованы и проверены вместе (`docs/implementation/THIRD_SERVICE_REPORT.md`,
`docs/implementation/integration-report.md`). Для этого задания это означает:

- публичный контракт зафиксирован: `openapi.yaml` версии **1.1.0**; добавлено поле
  `reminders_state` (`actual`, `pending`, `unavailable`) в `Document` и `DocumentListItem`,
  `next_reminder_at` может отсутствовать, у всех операций объявлен ответ `default` со схемой `Problem`,
  у `revokeInvite` добавлен ответ 409;
- интерфейс обязан показывать `reminders_state`: при `pending` — «напоминания пересчитываются»,
  при `unavailable` — «план временно недоступен», не выдавая устаревшее значение за актуальное;
- сессия выдаётся по `POST /api/v1/sessions` с непроверенной строкой `initData`; токен
  `vvs_<43 символа>` живёт 12 часов, после `401` нужно повторить запуск;
- цель запуска приходит в `session.start` (`document`, `organization`, `invite`, `none`) —
  это точка входа для диплинков напоминаний и приглашений;
- ссылка приглашения возвращается только в ответе на создание (`link_url`, `share_text`);
- ссылка экспорта календаря одноразовая: не более 3 скачиваний за 10 минут;
- для локальной проверки без MAX: `python3 scripts/sign_initdata.py --json` формирует тело
  запроса сессии; образ `edge` собирается из `deploy/edge/Dockerfile.frontend`, когда
  каталог `frontend/` появится в репозитории (сейчас отдаётся страница-заглушка).

## 1. Контекст продукта

«Вовремя» — мини-приложение MAX, в котором владелец малого бизнеса и его сотрудники ведут сроки лицензий, договоров, сертификатов и медосмотров. Бот MAX только открывает мини-приложение и присылает напоминания с кнопкой, ведущей в карточку документа.

## 2. Назначение frontend

Весь пользовательский интерфейс: запуск из MAX и идентификация, онбординг, реестр, карточка, формы, продление, участники и приглашения, настройки напоминаний, экспорт, удаление данных. Бизнес-правила (статусы, план напоминаний, права) вычисляет сервер; frontend их отображает и не дублирует, кроме валидации форм.

## 3. Реализуемые требования

BR-02, BR-04, BR-05; FR-01…FR-11, FR-13…FR-16, FR-18; NFR-01, NFR-03, NFR-05 (клиентская часть), NFR-06 (уведомление и удаление), NFR-09; PLAT-07, PLAT-08.

## 4. Карта экранов

`docs/frontend/frontend-architecture.md` §3 — нормативна (маршруты, данные, действия, роли).

## 5. Навигация

`docs/frontend/frontend-architecture.md` §4: `createMemoryRouter`; корневые экраны без BackButton; `replace` после создания и удаления; диплинк без истории → «Назад» ведёт на дашборд.

## 6. Стек

`docs/frontend/frontend-architecture.md` §1. `package.json` скрипты: `dev` (`vite`), `build` (`tsc -b && vite build`), `test` (`vitest`), `lint` (`eslint .`), `typecheck` (`tsc --noEmit`), `e2e` (`playwright test`), `gen-api` (`openapi-typescript ../openapi.yaml -o src/api/schema.d.ts`). `vite.config.ts`: `build.target = 'es2019'`, `server.proxy['/api'] = 'http://localhost:8080'`, тестовое окружение `jsdom`, `setupFiles: ['src/test/setup.ts']`.

## 7. Структура каталогов

`docs/frontend/frontend-architecture.md` §2 — точный список файлов. Ответственность ключевых файлов: `app/bootstrap.ts` — алгоритм §5 архитектуры; `app/router.tsx` — маршруты и интеграция BackButton; `api/client.ts` — `createClient<paths>()`, middleware авторизации, повтор после 401, разбор problem+json в `ApiError`; `session/sessionStore.ts` — токен, аккаунт, `start`; `platform/max/*` — единственный доступ к `window.WebApp`.

## 8. Интеграция MAX SDK

Подключение: в `index.html` до модуля приложения `<script src="https://st.max.ru/js/max-web-app.js"></script>`. Адаптер:

```ts
export type Platform = 'ios' | 'android' | 'desktop' | 'web';
export interface MaxBridge {
  readonly kind: 'max' | 'mock' | 'unavailable';
  readonly initData: string;
  readonly platform: Platform;
  readonly version: string;
  readonly backButton: { show(): void; hide(): void; onClick(cb: () => void): void; offClick(cb: () => void): void } | null;
  enableClosingConfirmation(): void;
  disableClosingConfirmation(): void;
  openLink(url: string): Promise<void>;
  openMaxLink(url: string): Promise<void>;
  shareMaxContent(p: { text: string; link: string }): Promise<void>;
  downloadFile(url: string, fileName: string): Promise<void>;
  readonly canScanQr: boolean;
  openCodeReader(): Promise<string>;
  haptic(kind: 'success' | 'error'): void;
}
export function getBridge(): Promise<MaxBridge>;
```

Правила поведения каждого метода, платформенные ограничения и запасные пути — `docs/max/max-integration-spec.md` §6. Ошибки Bridge (`{error: {code}}`) превращаются в `BridgeError(code)` и отправляются событием `bridge_error`. Имитация (`mockBridge.ts`, только при `VITE_MOCK_BRIDGE === 'true'`): `initData` подписывается WebCrypto HMAC-SHA256 по алгоритму spec §4 токеном `VITE_MOCK_BOT_TOKEN`; пользователь — `?mockUser=<id>` (по умолчанию 1001, имя «Тест <id>»); `start_param` — `?startapp=`; `platform = 'web'`, `version = 'mock'`; BackButton — плавающая кнопка «‹ Назад (MAX)»; `shareMaxContent` и `openCodeReader` — `window.prompt`; `downloadFile` и `openLink` — `window.open`; `haptic` — без действия; закрытие — `beforeunload`. Над приложением — полоса «Имитация MAX».

## 9. Жизненный цикл bootstrap

`docs/frontend/frontend-architecture.md` §5; тексты ошибок запуска — `docs/max/max-integration-spec.md` §3. Цель: первый экран ≤ 3 с (p75); событие `bootstrap_completed` с длительностью.

## 10. Аутентификация и сессия

Токен из `POST /sessions` — в памяти и `sessionStorage['vovremya.session']` вместе с `expires_at`; заголовок `Authorization: Bearer`; при 401 — одно обновление по `initData` и повтор запроса; выход — `DELETE /sessions/current`. Никаких cookie.

## 11. API-контракты

`openapi.yaml` 1.0.0; экраны и используемые операции — архитектура §3; сводка кодов — `docs/contracts/http-api.md`.

## 12. Типы запросов и ответов

Только сгенерированные `components['schemas']` из `src/api/schema.d.ts`; алиасы в `api/client.ts`: `type Document = components['schemas']['Document']` и т. п. Ручное описание типов API запрещено.

## 13. Серверное состояние

Архитектура §6 (ключи, `staleTime`, инвалидации, повторы).

## 14. Локальное состояние

Архитектура §7.

## 15. Формы и валидация

Архитектура §8; функции валидации — `shared/lib/validation.ts` и `features/documents/documentForm.ts`, покрыты тестами.

## 16. Состояния загрузки, ошибок, пустоты

Архитектура §9; компоненты `shared/ui/StateViews.tsx` (`LoadingView`, `EmptyView`, `ErrorView` с «Повторить»).

## 17. Адаптивность

Архитектура §11: 320–640 px колонка, `100dvh`, safe-area, закреплённая нижняя панель.

## 18. Тема

`MaxUI` с `colorScheme` из `prefers-color-scheme` и `platform` из Bridge; собственные цвета только для статусов, всегда с текстом.

## 19. Обработка ошибок

Архитектура §10 (таблица кодов); `ErrorBoundary` на уровне приложения показывает «Что-то пошло не так» и отправляет `api_error_shown` с кодом `render`.

## 20. Поведение при сбоях сети

Архитектура §10: баннер «Нет соединения», повтор GET при восстановлении, кнопка «Повторить» для изменений с тем же телом и UUID; офлайн-очереди нет.

## 21. Требования безопасности

Архитектура §13 и `docs/architecture/security.md`: без секретов; `initData` только в `POST /sessions`; внешние ссылки только `https://` через Bridge; запрет `dangerouslySetInnerHTML`; совместимость с CSP (`script-src 'self' https://st.max.ru`, без inline).

## 22. Компоненты, фичи, страницы

| Файл | Ответственность |
|---|---|
| `AppShell` | заголовок, контент, нижняя панель действий, баннер сети и канала напоминаний |
| `StatusBadge` | статус срока: цвет + текст + дни |
| `DateField` | ввод даты `YYYY-MM-DD` нативным `input type=date` |
| `OffsetChips` | выбор до 5 отступов и ввод своего |
| `ConfirmDialog` | подтверждение удаления и выхода |
| `ModelDataBadge` | пометка «Модельные данные справочника — сверяйте сроки с документом» |
| `OrganizationSwitcher` | список организаций из `me.memberships` |
| `DocumentListItem` | строка реестра |
| `CalendarExportButton` | два шага: «Подготовить файл» → «Скачать» (вызов `downloadFile` в клике) |
| Страницы | по карте экранов |

## 23. Тесты

| ID | Что проверяется | Критерий |
|---|---|---|
| T-FE-UTIL | `dates.ts` (сегодня в поясе, формат `DD.MM.YYYY`), `plural.ts`, `uuid.ts` (v4 по шаблону OpenAPI) | все случаи из таблицы тестов |
| T-FE-VAL | правила архитектуры §8 на границах | каждая граница — отдельный тест |
| T-FE-CMP | `StatusBadge` для 4 статусов, `OffsetChips` (максимум 5) | рендер и события |
| T-FE-PAGE | Дашборд, Реестр (фильтр, поиск, подгрузка), Карточка (роль viewer без кнопок), Форма (ошибки поля от сервера), Приглашение | MSW, проверки текста и вызовов |
| T-FE-API | 401 → обновление сессии → повтор; problem+json → `ApiError` | один повтор, корректный код |
| T-FE-MAX | `getBridge` выбирает max/mock/unavailable; BackButton показывается и скрывается по маршрутам; запасные пути share/download на `web` | вызовы шпионов |
| T-FE-NAV | маршрутизация по `start` для 4 видов | ожидаемый экран |
| T-FE-STATE | пустые, загрузка, ошибка для каждого экрана из архитектуры §9 | тексты присутствуют |
| T-FE-DUP | двойной клик «Сохранить» → один запрос; повтор после сетевой ошибки → тот же UUID | счётчик вызовов MSW |
| T-FE-A11Y | подписи полей, фокус на заголовке, `aria-live` | проверки Testing Library |
| T-E2E-01…06 | Playwright на локальном compose (`?mockUser`) — `docs/testing/test-strategy.md` | зелёный `make e2e` |

## 24. Фикстуры и заглушки

`src/test/fixtures.ts` — объекты по схемам OpenAPI (организация, 12 документов демо, участники); `src/test/msw/handlers.ts` — обработчики всех используемых операций с возвратом фикстур и сценариями ошибок (заголовок `x-mock-scenario: conflict|forbidden|network`); `VITE_MSW=true npm run dev` — работа без backend.

## 25. Сборка и запуск

```sh
cd frontend && npm ci
npm run gen-api            # после изменения openapi.yaml
npm run dev                # http://localhost:5173/?mockUser=1001, API проксируется на :8080
npm run build              # вывод размеров чанков; бюджет JS 200 KB gzip
npm test -- --run && npm run lint && npm run typecheck
```

## 26. Интеграция

```sh
docker compose up -d --build            # из корня: edge собирает frontend с имитацией Bridge
make e2e                                # Playwright против http://localhost:8080
```

Стенд: сборка с `VITE_MOCK_BRIDGE=false` выполняется в `deploy/edge/Dockerfile` при `APP_ENV=prod`; проверка в MAX — задачи MAX-03, MAX-04.

## 27. Решения, которые нельзя менять самостоятельно

React 18 + MAX UI + Vite; `createMemoryRouter`; токен в памяти и `sessionStorage` без cookie; только сгенерированные типы API; отсутствие глобальных сторов, кроме сессии; отсутствие офлайн-очереди; вызовы Bridge только через `platform/max`; тексты статусов; бюджет бандла; CSP без inline.

## 28. Definition of Done

Все экраны карты реализованы и доступны по ролям; тесты T-FE-* и T-E2E-* зелёные; `npm run build` укладывается в бюджет; `lint` и `typecheck` без ошибок; мини-приложение открывается и проходит AC-01…AC-10 на iOS, Android, desktop и web клиентах MAX (MAX-04); нет строки `devonly` в prod-бандле; review Разработчика B.
