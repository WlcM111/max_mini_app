# Редизайн интерфейса «Вовремя» (мини-приложение MAX)

Интерфейс переписан на собственную дизайн-систему. Логика работы с API, кэшем запросов, сессией
и MAX Bridge сохранена — изменены слой отображения и навигация.

## Главное
- Концепция «календарь, а не таблица»: каждый срок — листок отрывного календаря (цвет полосы = статус,
  крупное число = день), в карточке документа — обратный отсчёт и шкала периода действия.
- Навигация: нижняя панель разделов (Главная, Документы, Участники, Настройки); от 960 px — боковая панель.
- Плавающая кнопка «Добавить документ» / «Пригласить» сворачивается при прокрутке вниз.
- Крупный заголовок экрана и компактная панель при прокрутке; закреплённая панель действий внизу форм.
- Шторки-диалоги, всплывающие уведомления, скелетоны загрузки, пустые состояния с иллюстрацией.
- Анимации: появление экранов по направлению перехода, рост шкалы состояния на главной, отметки
  чек-листа, переключатели. При системной настройке «уменьшить движение» анимации отключаются.
- Адаптив: шрифты и отступы масштабируются от ширины окна, безопасные зоны iPhone, альбомная
  ориентация, планшет, десктоп, тёмная тема.

## Исправленные ошибки
- Кнопки «Участники / Настройки / Аккаунт» наезжали на нижнюю панель (кнопку «Добавить документ»
  нельзя было нажать) — заменены нижней навигацией.
- Синяя рамка вокруг заголовка после перехода.
- Невидимые поля ввода; поле даты выходило за край экрана на iPhone.
- Обрезанная кнопка «Добавить»; слипшийся текст «Михаил Зорин (вы)Владелец…».
- Дубль кнопки «Добавить документ» на пустой главной; сообщения в самом низу страницы.
- Маршрутизатор пересоздавался при каждом рендере.
- Черновик онбординга не сбрасывался при «Пропустить» — вторая организация получала занятый id;
  в переключатель организаций добавлено «Создать организацию».
- «Шаг 3 из 4» при подборе документов с главной; справочник запрашивался повторно.
- Экран «Сессия запуска устарела»: e2e-проверка находила два совпадения текста.

## Важные файлы
- Стили: `frontend/src/shared/styles/` — tokens.css, base.css, components.css, pages.css;
  шрифт цифр `fonts/vv-numerals.woff` (подмножество Lora, лицензия SIL OFL). Старый app.css удалён.
- Компоненты: `frontend/src/shared/ui/`; утилиты: `frontend/src/shared/lib/` (cx, format, navDirection).
- `vite.config.ts`: `assetsInlineLimit: 0` — шрифт отдаётся файлом (CSP запрещает data:-шрифты).
- `index.html`: `viewport-fit=cover` для безопасных зон iPhone.
- `@maxhub/max-ui` больше не используется, но оставлен в package.json, чтобы не менять package-lock.

## Проверка
Эмулятор MAX (рамка клиента, системная кнопка «Назад», безопасные зоны), 9 размеров окна
от 320 до 1280 px и тёмная тема. Автоматические проверки: выход за края, наложения, обрезанный
текст, перекрытие контента нижними панелями, размер кнопок — 0 замечаний. Скриншоты и отчёты —
в архиве vovremya_screens.zip.

Unit- и e2e-тесты в среде разработки не запускались (нет доступа к npm), их селекторы сверены вручную.
Перед сдачей выполните:

    cd frontend && npm ci && npm run build && npm run test && npm run e2e

## Изменённые файлы
Новые (24): shared/lib/cx.ts, shared/lib/format.ts, shared/lib/navDirection.ts, shared/styles/base.css, shared/styles/components.css, shared/styles/fonts/vv-numerals.woff, shared/styles/pages.css, shared/styles/tokens.css, shared/ui/Avatar.tsx, shared/ui/Banner.tsx, shared/ui/BrandMark.tsx, shared/ui/Button.tsx, shared/ui/DateLeaf.tsx, shared/ui/Fab.tsx, shared/ui/Field.tsx, shared/ui/Icon.tsx, shared/ui/Rows.tsx, shared/ui/Sheet.tsx, shared/ui/Splash.tsx, shared/ui/Steps.tsx, shared/ui/Switch.tsx, shared/ui/TabBar.tsx, shared/ui/Toast.tsx, shared/ui/backContext.ts

Изменены (37): app/App.tsx, app/ErrorBoundary.tsx, app/providers.tsx, app/router.tsx, features/documents/DocumentCardPage.tsx, features/documents/DocumentFormPage.tsx, features/documents/DocumentListItem.tsx, features/documents/DocumentsPage.tsx, features/documents/RenewPage.tsx, features/export/CalendarExportButton.tsx, features/invites/InviteAcceptPage.tsx, features/members/InvitePage.tsx, features/members/MembersPage.tsx, features/onboarding/DatesPage.tsx, features/onboarding/FeaturesPage.tsx, features/onboarding/OrganizationFormPage.tsx, features/onboarding/SuggestionsPage.tsx, features/onboarding/WelcomePage.tsx, features/organizations/DashboardPage.tsx, features/organizations/OrganizationSettingsPage.tsx, features/organizations/OrganizationSwitcher.tsx, features/settings/AccountPage.tsx, features/settings/SettingsPage.tsx, features/system/LaunchErrorPage.tsx, features/system/NotFoundPage.tsx, features/system/NotInMaxPage.tsx, main.tsx, platform/max/mockBridge.ts, shared/ui/AppShell.tsx, shared/ui/ConfirmDialog.tsx, shared/ui/DateField.tsx, shared/ui/ModelDataBadge.tsx, shared/ui/OffsetChips.tsx, shared/ui/StateViews.tsx, shared/ui/StatusBadge.tsx, test/render.tsx, frontend/vite.config.ts

Удалены (1): shared/styles/app.css
