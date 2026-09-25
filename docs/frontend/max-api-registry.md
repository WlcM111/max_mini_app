# Реестр использованных механизмов MAX

Версия 1.0.0 · 24.09.2026. Источник фактов — `docs/max/max-platform-facts.md`
(сводка официальной документации MAX, разделы S-INTRO, S-BRIDGE, S-VALID, S-UI-NPM)
и `docs/max/max-integration-spec.md` §3, §5, §6. Неподтверждённые методы не используются.

| Возможность | Факт | Где используется | Ограничения | Запасной путь |
|---|---|---|---|---|
| Подключение SDK `https://st.max.ru/js/max-web-app.js`, объект `window.WebApp` | F-08 | `index.html`, `platform/max/bridge.ts` | загрузка синхронно до модуля приложения | экран «Откройте из MAX» |
| `initData` (строка для серверной проверки) | F-09, F-10, F-13 | `bootstrap.ts` → `POST /sessions` | `initDataUnsafe` не используется | — |
| `platform`, `version` | F-15 | телеметрия, выбор стиля MAX UI | значения `ios/android/desktop/web` | `web` по умолчанию |
| `start_param` (диплинк `?startapp=`) | F-05, F-06 | цель запуска `session.start` | payload ≤ 512 символов, `A-Za-z0-9_-` | экран по умолчанию |
| `BackButton.show/hide/onClick/offClick` | F-17 | `platform/max/backButton.ts`, `app/router.tsx` | — | кнопка «‹» в шапке `AppShell` |
| `enableClosingConfirmation` / `disableClosingConfirmation` | F-18 | формы документа и продления при изменениях | — | без подтверждения |
| `openLink(url)` | F-19 | «Открыть источник» в карточке | только https, требует клика | `window.open` в имитации |
| `openMaxLink(url)` | F-19 | «Открыть чат с ботом», запасной шеринг | диплинки `https://max.ru/…` | копирование ссылки |
| `shareMaxContent({text, link})` | F-22 | отправка приглашения | требует клика | `https://max.ru/:share?text=…` (F-07), затем буфер обмена |
| `downloadFile(url, file_name)` | F-20 | экспорт `.ics` | работает только внутри клиента MAX, таймаут 60 с | `openLink(download_url)` |
| `openCodeReader(true)` | F-23 | «Сканировать QR» в форме документа | — | кнопка скрыта, если метода нет |
| `HapticFeedback.notificationOccurred` | F-25 | успех и ошибка сохранения | только iOS и Android | не вызывается |
| Библиотека `@maxhub/max-ui` (MIT) | F-29 | корневой компонент `MaxUI`, `Button`, `Input`, `Textarea`, `Switch` | React 18 требует версию 0.2.0 (C-FE-01) | — |

Не используются (и не эмулируются): `getLaunchContext` (F-16), `DeviceStorage`, `SecureStorage`
(F-24), `BiometricManager` (F-25), `NfcManager` (F-26), `requestContact` (F-28), `shareContent` (F-21).

Ошибки SDK приходят как `{ error: { code } }` (F-27) и приводятся к `BridgeError(code)`;
каждая неудача отправляется событием `bridge_error` в телеметрию.
