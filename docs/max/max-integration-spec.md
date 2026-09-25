# MAX integration specification

Версия 1.0.0 · 20.09.2026 · владелец: Разработчик A · reviewer: Разработчик B
Основание: [реестр фактов MAX](max-platform-facts.md) (официальная документация dev.max.ru, прочитана 20.09.2026). Идентификаторы F-xx, X-xx, U-xx — оттуда.

## 1. Назначение и границы

Интеграция с MAX состоит из трёх независимых каналов:

| Канал | Кто участвует | Направление | Механизм |
|---|---|---|---|
| K1. Запуск мини-приложения | Клиент MAX → frontend → core | клиент → сервер | MAX Bridge `window.WebApp.initData` → `POST /api/v1/sessions` → проверка подписи в core (F-09…F-14) |
| K2. События бота | MAX → edge → bot | MAX → сервер | Webhook `POST https://<домен>/max/webhook` (F-42…F-48) |
| K3. Сообщения бота | bot → MAX Bot API | сервер → MAX | `POST https://platform-api2.max.ru/messages?user_id=…` (F-49…F-51) |

Токен бота хранит только сервис `bot`. Сервис `core` получает производный ключ `hex(HMAC-SHA256("WebAppData", BOT_TOKEN))`: им можно проверить подпись данных запуска, но нельзя вызвать Bot API (ADR-007). Frontend не получает никаких секретов.

## 2. Идентичность пользователя

| Понятие | Значение | Источник доверия |
|---|---|---|
| Идентификатор пользователя MAX | `user.id` (int64) из проверенного `initData` | Подпись HMAC (F-13) |
| Имя | `user.first_name`, `user.last_name`, `user.username`, `user.language_code` | Подпись; обновляются при каждой новой сессии |
| Аккаунт продукта | `core.accounts` с `kind='max'`, уникальный `max_user_id` | Создаётся при первой сессии |
| Получатель сообщений бота | тот же `user.id`, передаётся в `EnqueueNotification.recipient_max_user_id` | Сопоставление 1:1 без дополнительной привязки |
| Не используются | `ip`, `photo_url`, `chat`, `query_id` (кроме журнала сессии), `initDataUnsafe` | — |

## 3. Bootstrap мини-приложения

1. `index.html` синхронно загружает `https://st.max.ru/js/max-web-app.js`, затем модуль приложения.
2. Адаптер `platform/max/bridge.ts` проверяет `window.WebApp` и непустой `initData`. Если их нет: при `VITE_MOCK_BRIDGE=true` подключается имитация (раздел 12), иначе показывается экран «Откройте приложение из чата с ботом в MAX» и отправляется событие `bootstrap_failed` с кодом `not_in_max`.
3. `POST /api/v1/sessions` с телом `{init_data, platform, app_version}`; таймаут клиента 10 с; одна автоматическая повторная попытка при сетевой ошибке через 1 с.
4. Ответ 201: токен сохраняется в памяти и в `sessionStorage['vovremya.session']` (ADR-009); `start` определяет первый экран (раздел 5).
5. `GET /api/v1/me` и `GET /api/v1/catalog` параллельно; маршрутизация: `start` → экран цели; нет организаций → онбординг; иначе дашборд последней выбранной организации (`localStorage['vovremya.lastOrg']`, только UUID).
6. `BackButton.onClick` подписывается один раз; видимость меняется при смене маршрута (раздел 6).
7. Телеметрия: `bootstrap_completed` с `duration_ms` от `performance.timeOrigin` до первого содержательного экрана.

Ошибки bootstrap: 401 `LAUNCH_DATA_EXPIRED` → экран «Сессия запуска устарела — закройте и снова откройте приложение»; 401 `LAUNCH_DATA_INVALID` → «Не удалось подтвердить запуск из MAX»; 429/503 → экран с кнопкой «Повторить» и отсчётом `Retry-After`; сеть → «Нет соединения» с повтором.

## 4. Проверка данных запуска на backend

Реализация: `services/core/internal/adapters/maxlaunch/verifier.go`, порт `ports.LaunchVerifier`.

Вход: строка `init_data` (1…4096 символов, иначе `VALIDATION_FAILED`), момент `now` (порт `Clock`), ключ `CORE_MAX_WEBAPP_SECRET_HEX` (32 байта).

Алгоритм (шаги 1–7 повторяют официальный, 8–11 — проектные проверки):

1. Разбить строку по `&`; каждую часть разделить по **первому** `=`. Часть без `=` → `LAUNCH_DATA_INVALID`.
2. Ключ, встретившийся более одного раза (включая `hash`) → `LAUNCH_DATA_INVALID`.
3. Извлечь `hash`; он обязан состоять ровно из 64 символов `[0-9a-f]`, иначе `LAUNCH_DATA_INVALID`.
4. Декодировать значения `url.PathUnescape` (плюс остаётся плюсом, F-14). Ошибка декодирования → `LAUNCH_DATA_INVALID`.
5. Отсортировать пары по ключу побайтно по возрастанию, исключив `hash`.
6. `launch_params` = пары `key=value`, соединённые `\n` (0x0A).
7. `expected = HMAC-SHA256(secret_key, launch_params)`; сравнить с `hex.DecodeString(hash)` через `hmac.Equal` (постоянное время). Несовпадение → `LAUNCH_DATA_INVALID`.
8. `auth_date` — целое число секунд. `now − auth_date > CORE_LAUNCH_MAX_AGE` (1 ч, F-11) → `LAUNCH_DATA_EXPIRED`; `auth_date − now > 60 с` → `LAUNCH_DATA_INVALID`.
9. `user` — JSON-объект с целым `id > 0` и строкой `first_name`; нарушение → `LAUNCH_DATA_INVALID`. Строковые поля обрезаются до ограничений таблицы `core.accounts`.
10. `start_param` разбирается по грамматике раздела 5; ошибка грамматики не отклоняет запуск, а даёт `start.kind = none`.
11. Защита от повторного использования: подпись ограничена временем (шаг 8); каждый вызов создаёт новую серверную сессию; частота создания ограничена 10 сессиями в минуту на `user.id` и 300 в минуту на IP-адрес (иначе 429 `RATE_LIMITED`). Сохранение `query_id` не используется для запрета повторов, так как документация не гарантирует его одноразовость для повторных загрузок страницы.

Результат: `LaunchIdentity{MaxUserID, FirstName, LastName, Username, LanguageCode, QueryID, AuthDate, StartParam}`.

Журналирование: исходная строка `init_data`, `hash` и `ip` в логи не пишутся. Метрика `vovremya_session_create_total{result}` с результатами `ok`, `invalid_signature`, `expired`, `future_auth_date`, `malformed`, `rate_limited`.

Тест-векторы — [test-vectors.md](test-vectors.md).

## 5. Грамматика start_param

Payload диплинка ограничен `^[A-Za-z0-9_-]{0,512}$` (F-05).

| Шаблон | Значение `StartTarget` | Кто формирует |
|---|---|---|
| пусто или отсутствует | `{kind: none}` | кнопка бота «Открыть приложение», кнопка меню |
| `doc_<uuid v4 в нижнем регистре>` | `{kind: document, document_id}` | кнопка напоминания |
| `org_<uuid v4>` | `{kind: organization, organization_id}` | сообщение «присоединился участник» |
| `inv_<43 символа base64url>` | `{kind: invite, invite_token}` | ссылка приглашения |
| любое иное значение | `{kind: none}` | — |

Доступ к цели проверяется обычными правилами ролей: чужой `doc_…` приводит к 404 на `GET /documents/{id}` и экрану «Документ недоступен».

## 6. Использование MAX Bridge

| Метод | Где | Платформы по документации | Поведение при недоступности или ошибке |
|---|---|---|---|
| `initData` | bootstrap | все | экран «Откройте из MAX» |
| `platform`, `version` | телеметрия, стиль MAX UI (`ios` → iOS, иначе Android) | все | `web` по умолчанию |
| `BackButton.show/hide/onClick/offClick` | маршрутизатор: показывать на всех маршрутах, кроме корневых (`/o/:orgId`, `/onboarding/welcome`) | все (ограничений не указано) | кнопка «Назад» в шапке приложения, если `window.WebApp.BackButton` отсутствует |
| `enableClosingConfirmation/disableClosingConfirmation` | формы документа, организации, продления при изменённых полях | все | без подтверждения |
| `openLink(url)` | «Открыть источник» в карточке, ссылка на правила | все, требует клика | обычная ссылка `target=_blank rel=noopener` в имитации |
| `openMaxLink(url)` | «Открыть чат с ботом» (`bot_chat_url`), запасной шеринг `https://max.ru/:share?text=…` | все | копирование ссылки в буфер обмена |
| `shareMaxContent({text, link})` | приглашение участника | не указаны ограничения, требует клика | 1) `openMaxLink(':share')` на iOS/Android/web; 2) копирование ссылки |
| `downloadFile(url, file_name)` | экспорт ICS | только внутри MAX, требует клика | при `platform=web` или ошибке `client.download_file.*` — `openLink(download_url)` |
| `openCodeReader(true)` | форма документа: «Сканировать QR» | все (ограничений не указано) | кнопка скрыта, если метода нет; ошибка → сообщение «Не удалось распознать код» |
| `HapticFeedback.notificationOccurred('success')` | успешное сохранение и продление | только iOS/Android (F-25) | не вызывается на `desktop`/`web` |
| `DeviceStorage`, `SecureStorage`, `BiometricManager`, `requestContact`, `NfcManager`, `shareContent`, `getLaunchContext` | не используются | — | — |

Правило клика: `openLink`, `downloadFile`, `shareMaxContent` вызываются синхронно в обработчике `onClick` без предварительного `await`; данные (ссылка экспорта, ссылка приглашения) готовятся заранее отдельным действием пользователя («Подготовить файл», «Создать приглашение»), после чего появляется кнопка, выполняющая сам вызов Bridge.

Обработка результата QR-кода: строка длиной ≤ 1024, начинающаяся с `https://`, записывается в `reference_url`; иная строка длиной ≤ 100 — в `number`; иначе сообщение «Код не похож на ссылку или номер документа».

## 7. Webhook бота (входящий контракт MAX → bot)

| Параметр | Значение |
|---|---|
| Публичный URL | `https://<домен>/max/webhook` (порт 443, F-44) |
| Маршрут | edge (Caddy) → `bot:8080` `POST /max/webhook`; иные методы → 404 на edge |
| Аутентичность | заголовок `X-Max-Bot-Api-Secret` равен `BOT_WEBHOOK_SECRET` (сравнение постоянного времени); иначе 401 без обработки |
| Размер тела | ≤ 256 KB (edge и bot), иначе 413 |
| Ответ | `200 {}` после фиксации транзакции; 503 — если БД недоступна (MAX повторит, F-45) |
| Время ответа | цель p99 ≤ 1 с; жёсткий таймаут обработчика 10 с (< 30 с по F-45) |
| Дедупликация | ключ `sha256(тело запроса)`; повтор → `200 {}` без повторных эффектов |
| Порядок событий | состояние получателя меняется, только если `timestamp` события ≥ `recipients.last_event_time` |

Разбор тела толерантный: читаются `update_type` (строка), `timestamp` (мс), идентификатор пользователя из `user.user_id` или `message.sender.user_id`, текст из `message.body.text`. Отсутствие нужных полей → `outcome='ignored'`, ответ 200. Реальные образцы событий (без текста пользователей) сохраняются в `services/bot/internal/adapters/webhook/testdata/` в задаче MAX-01.

| `update_type` | Эффект в `bot.recipients` | Ответ пользователю |
|---|---|---|
| `bot_started` | `active` | приветствие (раздел 15) |
| `bot_stopped`, `dialog_removed` | `stopped` | нет |
| `dialog_muted` | `muted` | нет |
| `dialog_unmuted` | `active` | нет |
| `message_created`, текст `/start` | `active` | приветствие |
| `message_created`, текст `/help` | `active` | справка |
| `message_created`, иной текст | `active` | подсказка «все действия в приложении», не чаще 1 раза в 10 минут на пользователя |
| остальные типы | нет | нет |

Подписка запрашивает `update_types = ["bot_started","bot_stopped","dialog_removed","dialog_muted","dialog_unmuted","message_created"]`.

Транзакция обработки: `INSERT bot.inbound_updates … ON CONFLICT DO NOTHING` → при вставке: `UPSERT bot.recipients` → при необходимости ответа `INSERT bot.outbound_messages` с ключом `reply:<hex dedupe_key>` → `COMMIT` → `200`.

## 8. Исходящие сообщения (bot → MAX)

Запрос для строки `bot.outbound_messages`:

```http
POST https://platform-api2.max.ru/messages?user_id=<recipient_max_user_id>
Authorization: <BOT_MAX_TOKEN>
Content-Type: application/json

{"text":"<text>","notify":<not silent>,"attachments":[{"type":"inline_keyboard","payload":{"buttons":[[<button 1>],[<button 2>]]}}]}
```

`format` не передаётся: текст — обычный, экранирование разметки не требуется. `attachments` отсутствует, если кнопок нет.

| Кнопка в очереди | JSON для MAX при `BOT_OPEN_APP_BUTTON_KIND=link` (по умолчанию) | при `open_app` (только после MAX-02, X-01) |
|---|---|---|
| `action=open_app`, payload `P` | `{"type":"link","text":T,"url":"https://max.ru/<username>?startapp=P"}` (для пустого P — `…?startapp`) | `{"type":"open_app","text":T,"web_app":"<username>","payload":P}` |
| `action=url` | `{"type":"link","text":T,"url":U}` | то же |

Классификация ответа MAX:

| Ответ | Новый статус | `last_error_code` | Дополнительно |
|---|---|---|---|
| 200 | `sent` | — | `max_message_id` = `message.body.mid`, если поле есть; `recipients.last_delivery_at` |
| 400, 405 | `failed` | `max_4xx` | ошибка формирования запроса, метрика и лог уровня error |
| 401 | `retry_wait` | `max_401` | сигнал о недействительном токене, лог error один раз в минуту |
| 403, 404 | `failed` | `recipient_unreachable` | `recipients.state = unreachable` |
| 429 | `retry_wait` | `max_429` | глобальная скорость снижается вдвое на 60 с; учитывается `Retry-After`, если он пришёл |
| 500, 503, иные 5xx | `retry_wait` | `max_5xx` | — |
| таймаут 10 с, сетевая ошибка | `retry_wait` | `timeout` / `network` | возможен дубль (доставка «хотя бы один раз») |

Перед отправкой: если `recipients.state = stopped`, сообщение переводится в `failed` с кодом `recipient_stopped` без вызова MAX.

Повторы: задержка попытки n = случайное значение в [0; min(10 мин, 5 с × 2^(n−1))]; не более 8 попыток; после этого `failed`. Если `now > not_after` до отправки — `expired`.

Ограничения скорости: глобальный токен-бакет 20 запросов/с (`BOT_GLOBAL_RPS`, официальный предел 30, F-43) на все вызовы Bot API; для каждого получателя не чаще одного сообщения в 600 мс (официальный предел 2 в секунду, F-50).

## 9. Подписка webhook и её восстановление

`SubscriptionKeeper` (только `BOT_MODE=live`) при старте и каждые 10 минут:

1. `GET /subscriptions`; ошибка → метрика `vovremya_max_subscription_ok=0`, повтор через 30 с.
2. Если среди подписок нет `BOT_WEBHOOK_PUBLIC_URL` → `POST /subscriptions {url, update_types, secret}`; при `success=false` — лог error с полем `message`.
3. Лишние подписки на другие URL журналируются (warning) и не удаляются автоматически: формат параметров `DELETE /subscriptions` не проверен.

Так восстанавливается подписка после автоматической отписки через 8 часов недоступности (F-45).

## 10. Профиль бота и диплинки

При старте `live` бот вызывает `GET /me` и кэширует `username`, `name`. До успешного ответа `GetBotProfile` возвращает `UNAVAILABLE`, отправка сообщений с кнопками откладывается (строки остаются в `queued`). Повтор `GET /me` — каждые 30 с. В режиме `stub` профиль: `username = BOT_STUB_USERNAME`.

`open_app_link_template = "https://max.ru/<username>?startapp={payload}"`, `chat_url = "https://max.ru/<username>"`. core подставляет только payload, соответствующий разделу 5.

## 11. TLS и сертификаты

| Направление | Требование | Решение |
|---|---|---|
| MAX → наш webhook | доверенный УЦ или Минцифры, полная цепочка, порт 443 (F-44) | Caddy получает сертификат ACME (Let's Encrypt) для `EDGE_SITE_ADDRESS` |
| bot → `platform-api2.max.ru` | добавить сертификат Минцифры в доверенные (F-40) | пул доверия = системные корни Alpine + `deploy/ca/russian_trusted_root_ca.pem` + `russian_trusted_sub_ca.pem` (файл `BOT_MAX_EXTRA_CA_FILE`) |
| клиент MAX → мини-приложение | https (F-03) | тот же сертификат Caddy |

Файлы Минцифры скачиваются в задаче INF-02 с портала https://www.gosuslugi.ru/crt, их SHA-256-отпечатки сверяются с опубликованными на портале и записываются в [runbook](../operations/compose-and-runbook.md).

## 12. Режимы окружения

| Аспект | `APP_ENV=local` (по умолчанию) | `APP_ENV=prod` |
|---|---|---|
| Frontend Bridge | имитация `mockBridge.ts`: `initData` подписывается в браузере dev-токеном через WebCrypto; пользователь задаётся `?mockUser=<id>`, payload — `?startapp=<payload>` | настоящий `max-web-app.js` |
| Сборка frontend | `VITE_MOCK_BRIDGE=true` | `false`; сборка падает, если в бандле найдена строка `devonly` |
| bot | `BOT_MODE=stub`: сообщения не уходят в MAX, пишутся в лог и в кольцевой буфер 500 записей, доступный на `GET http://bot:8081/debug/stub/messages` | `BOT_MODE=live`: webhook, Bot API, подписка |
| Ключ проверки initData | производный от `devonly-local-bot-token` | производный от токена организаторов |
| Секреты | значения `devonly_*` | запуск прерывается, если любое секретное значение начинается с `devonly` или равно dev-ключу |

Запуск webhook с машины разработчика допускается только через HTTPS-туннель с доверенным сертификатом на порту 443 (F-44) и отдельным тестовым ботом; основной путь проверки — стенд (INF-01).

## 13. Настройка production

1. Получить токен бота у организаторов и уточнить способ привязки URL мини-приложения (U-01, блокер B-01).
2. Подготовить VPS с публичным IPv4, Docker Engine 27+ с Compose v2, открыть 80/tcp и 443/tcp; A-запись домена на IP сервера.
3. Положить сертификаты Минцифры в `deploy/ca/` (раздел 11).
4. `python3 scripts/generate_prod_env.py --domain <домен> --acme-email <почта> --token-file <файл>`.
5. `docker compose up -d --build`; убедиться, что `https://<домен>/` открывается, `GET /api/v1/me` без токена отвечает 401.
6. Проверить в логах bot сообщения `bot profile loaded` и `webhook subscription ensured`.
7. В настройках бота указать URL мини-приложения `https://<домен>/` и вид кнопки «Открыть».
8. Написать боту `/start`: пришло приветствие с кнопкой; нажать кнопку: открылось мини-приложение; пройти критерии раздела 16.
9. Выпустить токены проверяющих: `docker compose exec core /app/core review-token issue --login reviewer_editor --role editor --ttl 336h` и то же для `reviewer_viewer` с ролью `viewer` (ADR-016).

## 14. Безопасность интеграции

| Угроза | Мера |
|---|---|
| Подделка `initData` пользователем | HMAC-проверка на backend; `initDataUnsafe` не отправляется и не используется |
| Повтор перехваченного `initData` | срок 1 ч, лимит частоты сессий, HTTPS |
| Подделка webhook | секрет `X-Max-Bot-Api-Secret`, сравнение постоянного времени |
| Утечка токена бота | токен только в `bot`; в core — производный ключ; токен не логируется; `.env` с правами 600 |
| Открытие чужих данных по диплинку | доступ к цели проверяется по роли; payload не содержит секретов, кроме одноразового токена приглашения (хранится хешем, срок 72 ч) |
| Фишинговые ссылки в документах | `reference_url` только `https://`, открывается через `openLink` во внешнем браузере |
| Встраивание мини-приложения чужим сайтом | CSP `frame-ancestors` из `EDGE_FRAME_ANCESTORS` |

## 15. Тексты сообщений бота

| Вид | Текст | Кнопки |
|---|---|---|
| `welcome` | «Здравствуйте! Я «Вовремя» — напоминаю о сроках лицензий, договоров и других документов бизнеса. Откройте приложение, чтобы добавить документы. Напоминания придут в этот чат.» | «Открыть приложение» → payload пусто |
| `help` | «Что я умею: веду список документов с датами окончания; заранее напоминаю о сроках — по умолчанию за 30, 7 и 1 день; помогаю вести сроки вместе с сотрудниками. Настройки напоминаний — в приложении, раздел «Настройки».» | «Открыть приложение» |
| подсказка | «Я не читаю сообщения — все действия доступны в приложении.» | «Открыть приложение» |
| `reminder` (формирует core) | «Через {N} {дней} заканчивается срок: «{title}». Организация: {org_name}. Срок до {DD.MM.YYYY}.»; при N = 0 — «Сегодня заканчивается срок: …» | «Открыть документ» → `doc_<uuid>` |
| `member_joined` (формирует core, получатель — владелец) | «В организацию «{org_name}» присоединился участник {first_name} с ролью «{роль}».» | «Открыть организацию» → `org_<uuid>` |

Склонение «день/дня/дней» — функция `domain.PluralDays` в core. Длина текста ≤ 4000 символов (F-49): `title` и `org_name` ограничены 200 и 100 символами.

## 16. Критерии приёмки MAX-интеграции

| ID | Проверка | Способ | Критерий прохождения |
|---|---|---|---|
| AC-MAX-01 | Открытие из кнопки бота на Android, iOS, desktop и web | ручной прогон на устройствах команды | на всех четырёх платформах за ≤ 5 с открыт дашборд или онбординг, в логах core `session.created` |
| AC-MAX-02 | Отказ при изменённом `initData` | TV-2 через curl на стенде | 401 `LAUNCH_DATA_INVALID` |
| AC-MAX-03 | Отказ при устаревшем `initData` | TV-1 с `auth_date` старше 1 ч | 401 `LAUNCH_DATA_EXPIRED` |
| AC-MAX-04 | Диплинк `doc_<uuid>` открывает карточку | ссылка из напоминания | открыта карточка нужного документа |
| AC-MAX-05 | Напоминание доставлено | документ со сроком через 1 день и отступом 1, время уведомлений через 3 минуты | сообщение пришло в чат с ботом с кнопкой; нажатие открывает карточку |
| AC-MAX-06 | Приветствие на `/start` | написать боту | приветствие с кнопкой за ≤ 10 с |
| AC-MAX-07 | Webhook отвергает запрос без секрета | `curl -X POST https://<домен>/max/webhook -d '{}'` | 401, в `bot.inbound_updates` новых строк нет |
| AC-MAX-08 | Повтор webhook не дублирует ответ | повторная отправка того же тела с верным секретом | одно исходящее сообщение |
| AC-MAX-09 | Кнопка «Назад» MAX | карточка документа → «Назад» | возврат к списку, на корневом экране кнопка скрыта |
| AC-MAX-10 | Шеринг приглашения | «Пригласить» → «Отправить в MAX» | открыт экран выбора чата MAX с текстом и ссылкой; в веб-версии работает запасной путь |
| AC-MAX-11 | Экспорт ICS | «Экспорт в календарь» на Android | файл `.ics` сохранён; в веб-версии открыт по ссылке |
| AC-MAX-12 | Состояние канала | остановить бота в настройках MAX | в «Настройках» мини-приложения статус «Бот остановлен» и кнопка «Открыть чат с ботом» |
