# Факты о платформе MAX: реестр проверки

Версия 1.0.0 · дата проверки источников: 20.09.2026 · владелец: Разработчик A (MAX integration)

Документ фиксирует, какие возможности MAX подтверждены официальной документацией, какие известны только из сторонних источников и какие не подтверждены. Проектные решения опираются только на строки со статусом «официально». Остальные закрыты спайками MAX-01…MAX-04 (см. [план](../plan/team-plan.md)) или обходятся запасным вариантом.

## 1. Источники

| Код | Источник | Адрес | Статус |
|---|---|---|---|
| S-INTRO | Подключение мини-приложения | https://dev.max.ru/docs/webapps/introduction | официальный, прочитан 20.09.2026 |
| S-BRIDGE | MAX Bridge | https://dev.max.ru/docs/webapps/bridge | официальный, прочитан 20.09.2026 |
| S-VALID | Валидация данных | https://dev.max.ru/docs/webapps/validation | официальный, прочитан 20.09.2026 |
| S-API | Обзор API | https://dev.max.ru/docs-api | официальный, прочитан 20.09.2026 |
| S-SUBS | POST /subscriptions | https://dev.max.ru/docs-api/methods/POST/subscriptions | официальный, прочитан 20.09.2026 |
| S-MSG | POST /messages | https://dev.max.ru/docs-api/methods/POST/messages | официальный, прочитан 20.09.2026 |
| S-UPD | Объект Update | https://dev.max.ru/docs-api/objects/Update | официальный, прочитан 20.09.2026 |
| S-MOBJ | Объект Message | https://dev.max.ru/docs-api/objects/Message | официальный, прочитан 20.09.2026 |
| S-HACK | Сайт хакатона | https://hackathon-max.vk.company/ | официальный сайт организаторов, прочитан 20.09.2026 |
| S-UI-NPM | Пакет @maxhub/max-ui | https://www.npmjs.com/package/@maxhub/max-ui , https://github.com/max-messenger/max-ui | реестр npm и репозиторий организации max-messenger |
| S-3P | Сторонние SDK (pkg.go.dev demen1n/maxbot, docs.rs maxbot, green-api) | pkg.go.dev, docs.rs, green-api.com | сторонние, не нормативные |
| S-CASE | Кейс трека «Эффективный бизнес» (PDF) | вложение пользователя | нормативный для сдачи |
| S-CERT | Сертификаты НУЦ Минцифры | https://www.gosuslugi.ru/crt | официальный портал; прямые файлы по ссылкам из инструкций банков (gu-st.ru) |

## 2. Мини-приложение и MAX Bridge (подтверждено официально)

| ID | Факт | Источник | Где используется |
|---|---|---|---|
| F-01 | Мини-приложение работает только внутри чат-бота MAX и не существует автономно | S-INTRO | ADR-002, ADR-008 |
| F-02 | URL мини-приложения задаётся в настройках бота на платформе MAX для партнёров (business.max.ru → Чат-боты → ⋮ → Настройки); вид кнопки: «Открыть», «Старт», «Играть» или без названия | S-INTRO | MAX-01, runbook |
| F-03 | Требования к URL: не длиннее 1024 символов, только https://, допустимые символы — латиница, цифры, точка, дефис, без пробелов | S-INTRO | Используем URL вида `https://<домен>/` без пути и параметров |
| F-04 | Подключение к платформе — для юрлиц, ИП и самозанятых — резидентов РФ | S-INTRO | Противоречие C-03: у команды нет доступа к кабинету, токен выдают организаторы (S-HACK) |
| F-05 | Диплинк: `https://max.ru/<botName>?startapp=<payload>`; payload до 512 символов, только `A-Z a-z 0-9 _ -`; при нарушении payload удаляется | S-INTRO | Кнопки напоминаний, приглашения |
| F-06 | payload доступен как `initDataUnsafe.start_param`, внутри `initData` и в GET-параметре `WebAppStartParam` | S-INTRO, S-BRIDGE | core разбирает `start_param` из проверенных данных |
| F-07 | Диплинк шеринга `https://max.ru/:share?text=…` работает на iOS, Android и в веб-версии; на десктопе — в разработке | S-INTRO | Запасной путь шеринга приглашения |
| F-08 | Подключение: `<script src="https://st.max.ru/js/max-web-app.js">`, объект `window.WebApp` доступен без инициализации | S-BRIDGE | index.html, CSP `script-src` |
| F-09 | `initData` — URL-кодированная строка для серверной валидации; `initDataUnsafe` нельзя использовать для валидации | S-BRIDGE | ADR-007 |
| F-10 | Поля данных запуска: `query_id`, необязательный `ip`, `auth_date` (Unix, секунды), `hash`, `user{id, first_name, last_name, username, language_code, photo_url}`, `chat{id, type: DIALOG/CHAT/CHANNEL}`, `start_param` | S-BRIDGE, S-VALID | Account, StartTarget |
| F-11 | Рекомендуемый интервал действительности `auth_date` — 1 час | S-BRIDGE | `CORE_LAUNCH_MAX_AGE=1h` |
| F-12 | Параметры платформы передаются во фрагменте URL после `#`: `WebAppData=…&WebAppPlatform=…&WebAppVersion=…`; в хеш входит только `WebAppData` | S-VALID, S-BRIDGE | Маршрутизация фронтенда в памяти (ADR-003), platform — недоверенная метаинформация |
| F-13 | Алгоритм: разобрать пары, проверить единственность параметров и `hash`, URL-декодировать значения, отсортировать по ключу, склеить `key=value` через `\n`; `secret_key = HMAC-SHA256(key="WebAppData", message=BOT_TOKEN)`; подпись `hex(HMAC-SHA256(secret_key, launch_params))` сравнить с `hash` | S-VALID | ADR-007, [спецификация](max-integration-spec.md#4-проверка-данных-запуска-на-backend) |
| F-14 | Официальный пример декодирует значения `decodeURIComponent`, то есть `+` не превращается в пробел | S-VALID | В Go использовать `url.PathUnescape`, не `QueryUnescape` |
| F-15 | `WebApp.platform`: `ios`, `android`, `desktop`, `web`; `version` вида `26.2.8`; `deviceName` | S-BRIDGE | Телеметрия, выбор стиля MAX UI |
| F-16 | `getLaunchContext()` доступен на Android ≥ 26.19.2 и iOS ≥ 26.20.0 | S-BRIDGE | Не используется |
| F-17 | `BackButton.show/hide/isVisible/onClick/offClick` | S-BRIDGE | Навигация назад |
| F-18 | `enableClosingConfirmation()` / `disableClosingConfirmation()` | S-BRIDGE | Формы с несохранёнными изменениями |
| F-19 | `openLink(url)` открывает внешний браузер, требует клика пользователя; `openMaxLink(url)` открывает диплинки `https://max.ru/…` внутри MAX, иные ссылки — во внешнем браузере | S-BRIDGE | Ссылка на источник, чат с ботом |
| F-20 | `downloadFile(url, file_name)`: только https, требует клика, работает в мини-приложении, открытом в мессенджере MAX, «в браузере метод не работает»; ошибки `client.download_file.invalid_params`, `client.download_file.request_timeout` (60 с) | S-BRIDGE | Экспорт ICS |
| F-21 | `shareContent({text, link})` — нативный шеринг iOS/Android, в веб-приложении не поддерживается | S-BRIDGE | Не используется |
| F-22 | `shareMaxContent({text, link})` или `shareMaxContent({mid, chatType})` — экран шеринга внутри MAX, требует клика; режим медиа — пересылка сообщения бота | S-BRIDGE, S-INTRO | Шеринг приглашения (режим text/link) |
| F-23 | `openCodeReader(fileSelect = true)` возвращает строку распознанного QR-кода | S-BRIDGE | Заполнение ссылки/номера документа |
| F-24 | `DeviceStorage` и `SecureStorage` не поддерживаются веб-приложением; `SecureStorage` — до 10 ключей на пользователя | S-BRIDGE | Не используются (ADR-009) |
| F-25 | `BiometricManager` и `HapticFeedback` не поддерживаются десктоп- и веб-клиентом | S-BRIDGE | Haptic — только при `platform ∈ {ios, android}` |
| F-26 | `NfcManager` — только Android | S-BRIDGE | Не используется |
| F-27 | Методы возвращают Promise; при ошибке `reject({ error: { code } })` | S-BRIDGE | Адаптер `platform/max` |
| F-28 | `requestContact()` возвращает номер с хешем для проверки | S-BRIDGE | Не используется: телефон не нужен продукту |
| F-29 | Библиотека MAX UI: пакет `@maxhub/max-ui`, лицензия MIT, React 18+, TypeScript; стили `@maxhub/max-ui/dist/styles.css`, корневой компонент `MaxUI` | S-UI-NPM | ADR-003 |

## 3. Bot API (подтверждено официально)

| ID | Факт | Источник | Где используется |
|---|---|---|---|
| F-40 | Базовый домен `https://platform-api2.max.ru`; токен только в заголовке `Authorization: <token>`; в доверенные нужно добавить сертификат Минцифры | S-API | `BOT_MAX_API_BASE_URL`, `BOT_MAX_EXTRA_CA_FILE` |
| F-41 | Коды ответов: 200, 400, 401, 404, 405, 429, 503; на страницах методов также 500 | S-API, S-MSG | Классификация ошибок доставки |
| F-42 | Для production — только Webhook; Long Polling ограничен по скорости и сроку хранения событий и не подходит для production; одновременно оба способа использовать нельзя; при активной подписке Long Polling не работает | S-API, S-SUBS | ADR-008: только webhook, Long Polling не реализуется |
| F-43 | Максимум 30 запросов в секунду на `platform-api2.max.ru` | S-API | Глобальный лимитер bot 20 rps |
| F-44 | Webhook: HTTPS, только порт 443, сертификат доверенного УЦ или Минцифры, совпадение имени с CN/SAN, полная цепочка; с 25 мая 2026 HTTP и самоподписанные сертификаты не поддерживаются | S-SUBS, S-API | Caddy с ACME на 443 |
| F-45 | Webhook должен вернуть HTTP 200 за 30 секунд; иначе до 10 повторов с интервалом 60 с × 2,5 на каждую попытку; если 8 часов нет успешного ответа — автоматическая отписка | S-SUBS | Webhook отвечает после записи в БД; SubscriptionKeeper восстанавливает подписку |
| F-46 | `secret` (`^[a-zA-Z0-9_-]{5,256}$`) передаётся в заголовке `X-Max-Bot-Api-Secret` каждого запроса | S-SUBS | Проверка подлинности webhook |
| F-47 | `POST /subscriptions`: тело `{url, update_types?, secret?}`; ответ `{success, message?}`; есть `GET /subscriptions` и `DELETE /subscriptions` | S-SUBS, S-API | SubscriptionKeeper |
| F-48 | Типы событий: `bot_added`, `bot_started`, `bot_stopped`, `bot_removed`, `chat_title_changed`, `dialog_cleared`, `dialog_muted`, `dialog_unmuted`, `dialog_removed`, `message_callback`, `message_created`, `message_edited`, `message_removed`, `comment_created`, `comment_edited`, `comment_removed`, `user_added`, `user_removed`; поле `timestamp` — миллисекунды | S-UPD | RecipientState |
| F-49 | `POST /messages?user_id=<int64>` или `?chat_id=<int64>`, `disable_link_preview`; тело `text` (до 4000 символов), `attachments`, `link`, `notify` (по умолчанию true), `format` (`markdown`/`html`); ответ `{message: Message}` | S-MSG | MaxClient.SendMessage |
| F-50 | Не более 2 сообщений в секунду в один диалог, чат или канал | S-MSG | Лимитер на получателя |
| F-51 | Inline-клавиатура: до 210 кнопок, 30 рядов, до 7 в ряду (до 3 для `link`, `open_app`, `request_geo_location`, `request_contact`); типы `callback`, `link` (URL до 2048), `request_contact`, `request_geo_location`, `open_app`, `message`, `clipboard`; кнопки не пересылаются вместе с сообщением | S-API | Не более 3 кнопок, по одной в ряду |
| F-52 | `GET /me` возвращает `user_id`, `name`, `username`, `is_bot`, `last_activity_time` | S-API | GetBotProfile |
| F-53 | `PATCH /me/commands` задаёт команды бота | S-API | Команды `/start`, `/help` |
| F-54 | `GET /chats` не поддерживается с июня 2026; `POST /chats/{chatId}/members` ограничен с 09.09.2026 и удаляется 30.09.2026 | S-API | Не используются |
| F-55 | `Message`: `sender`, `recipient`, `timestamp` (мс), `link`, `body` (MessageBody), `stat`, `url` | S-MOBJ | Идентификатор сообщения сохраняется из `body` |

## 4. Сведения о хакатоне (официальный сайт)

| ID | Факт | Источник | Следствие |
|---|---|---|---|
| H-01 | Команда — от 3 до 4 человек | S-HACK | Противоречие C-01 с условием «2 разработчика» |
| H-02 | Онлайн-тур 15.09–30.09, оценка 30.09–14.10, финал 29.10 в Казани (Kazan Digital Week) | S-HACK | Дедлайн плана — 29.09.2026 из запроса, 30.09 не используется как резерв |
| H-03 | Юрлицо не требуется: каждой команде выдаётся уникальный токен для разработки на платформе MAX | S-HACK (ответ FAQ в поисковом индексе) | Токен бота — входной параметр, выдаётся организаторами |
| H-04 | Правила опубликованы по ссылке на cloud.mail.ru | S-HACK | Текст правил не прочитан (страница требует JavaScript); требования сдачи взяты из PDF кейса |

## 5. Только сторонние источники (не нормативно)

| ID | Сведение | Источник | Как обращаемся |
|---|---|---|---|
| X-01 | Кнопка `open_app` имеет поля `web_app` (ник бота или ссылка), `contact_id`, полезная нагрузка — `payload` или `app_payload` (источники расходятся) | S-3P | По умолчанию кнопка `link` с официальным диплинком (F-05); `open_app` включается флагом после MAX-02 |
| X-02 | Идентификатор сообщения — поле `mid` внутри `body` | S-3P, косвенно S-INTRO | Сохраняем `body.mid`, если поле есть; продукт от него не зависит |

## 6. Не подтверждено и проверяется спайками

| ID | Вопрос | Проверка | Запасной вариант |
|---|---|---|---|
| U-01 | Как привязать URL мини-приложения к боту, токен которого выдали организаторы (доступ к кабинету партнёра) | MAX-01: письмо на support@hackathon-max.vk.company в первый день | Ссылка из бота кнопкой `link` на диплинк не работает без привязки — блокер B-01 |
| U-02 | Точный origin веб-клиента MAX, встраивающего мини-приложение (для `frame-ancestors`) | MAX-03: открыть в web.max.ru, записать `document.referrer` и `location.ancestorOrigins` в телеметрию | `EDGE_FRAME_ANCESTORS` меняется без пересборки |
| U-03 | API темы оформления в Bridge не документирован | — | `prefers-color-scheme` |
| U-04 | Работает ли `downloadFile` в веб-клиенте MAX | MAX-03 | Для `platform=web` — `openLink(download_url)` |
| U-05 | Доставляет ли `open_app`-кнопка payload в `start_param` | MAX-02 | Кнопка `link` с диплинком |
| U-06 | Сохраняются ли `sessionStorage`/`localStorage` в WebView мобильных клиентов | MAX-03 | Потеря хранилища = повторная сессия по `initData` |
| U-07 | Диапазоны IP-адресов отправителя webhook | — | Проверка `X-Max-Bot-Api-Secret` |
| U-08 | Требования `max-web-app.js` к CSP (inline-скрипты, `connect-src`) | MAX-03: отчёт CSP в консоли | Расширить CSP точечно, записать в ADR-011 |
| U-09 | Точная структура DATA-API.yaml сверх 9 пунктов кейса | Вопрос организаторам в MAX-01 | Файл по 9 пунктам кейса |
