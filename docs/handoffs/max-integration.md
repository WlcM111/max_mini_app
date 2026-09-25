# Задание на MAX integration

Версия 1.0.0 · 20.09.2026 · исполнитель: Разработчик A · reviewer: Разработчик B. Комплект — `handoff-max.zip`. Нормативная спецификация — `docs/max/max-integration-spec.md` (далее spec), реестр фактов — `docs/max/max-platform-facts.md`.

| № | Обязательный пункт | Содержание |
|---|---|---|
| 1 | Источник официальной документации | https://dev.max.ru/docs/webapps/introduction, /docs/webapps/bridge, /docs/webapps/validation, https://dev.max.ru/docs-api и страницы методов (реестр фактов §1) |
| 2 | Версия и дата | документация прочитана 20.09.2026; Bridge — скрипт `https://st.max.ru/js/max-web-app.js` без версии в URL; спецификация 1.0.0 |
| 3 | Bootstrap | spec §3 |
| 4 | Identity | spec §2 |
| 5 | Проверка подписи | spec §4, тест-векторы `docs/max/test-vectors.md` |
| 6 | Проверка на backend | core, `internal/adapters/maxlaunch/verifier.go` (handoff core, раздел 18) |
| 7 | Методы SDK | spec §6 |
| 8 | Поведение окружений | spec §12 (local/prod), §6 (платформы iOS, Android, desktop, web) |
| 9 | Ошибки | spec §3 (запуск), §6 (Bridge), §8 (Bot API) |
| 10 | Безопасность | spec §14 |
| 11 | Локальная разработка | spec §12: имитация Bridge, `BOT_MODE=stub`, туннель только с тестовым ботом |
| 12 | Production setup | spec §13 |
| 13 | Критерии приёмки | spec §16 (AC-MAX-01…12) |

## Задачи

| ID | Шаги | Результат |
|---|---|---|
| MAX-01 (20–21.09) | 1) письмо организаторам с вопросами Q-01…Q-04 (conflicts-assumptions); 2) `curl -H "Authorization: $TOKEN" https://platform-api2.max.ru/me` — записать `username`; 3) `POST /messages?user_id=<свой id>` с текстом и кнопкой `link` — проверить доставку; 4) на стенде (`BOT_MODE=live`) убедиться в `webhook subscription ensured`; 5) написать боту `/start`, `/help`, текст, заглушить и включить уведомления — сохранить тела событий из лога отладки (временный `LOG_LEVEL=debug`) в `testdata`, заменив имена на «Тест»; 6) проверить, нужен ли сертификат Минцифры (A-06), и обновить реестр фактов | статусы U-01, A-04, A-06 обновлены; ≥ 3 файлов `testdata` |
| MAX-02 (26.09) | отправить себе сообщения с кнопкой `link` на `?startapp=doc_<uuid>` и кнопкой `open_app` по X-01; открыть на 4 платформах; записать, приходит ли `start_param` | решение по `BOT_OPEN_APP_BUTTON_KIND`; обновлён X-01 |
| MAX-03 (22.09) | открыть мини-приложение из бота на iOS, Android, desktop, web; для web записать `document.referrer` и `location.ancestorOrigins` (событие `bridge_error` с кодом `diag`), ошибки CSP в консоли, работу `downloadFile`, сохранение `sessionStorage` после сворачивания | таблица результатов в реестре фактов (U-02, U-04, U-06, U-08); `EDGE_FRAME_ANCESTORS` в `.env` стенда |
| MAX-04 (27–28.09) | пройти AC-MAX-01…12 на кандидате релиза | протокол в `docs/testing/acceptance.md` |
