# Эксплуатация: Docker Compose и runbook

Версия 1.0.0 · 20.09.2026. Для участников без опыта администрирования каждое действие описано командой и ожидаемым результатом.

## 1. Локальный запуск (проверяющие и разработка)

| Действие | Команда | Результат |
|---|---|---|
| Запуск всего | `docker compose up -d --build` | через 1–3 минуты `docker compose ps` показывает `healthy` у postgres, bot, core, edge и `exited (0)` у миграций |
| Открыть мини-приложение | браузер: `http://localhost:8080/?mockUser=1001` | онбординг от имени тестового пользователя 1001 (имитация MAX) |
| Логи | `docker compose logs -f core bot` | JSON-строки с `request_id` |
| Остановка | `docker compose down` | контейнеры удалены, данные сохранены в томе `pgdata` |
| Повторный запуск | `docker compose up -d` | данные на месте |
| Полный сброс данных | `docker compose down -v` | удалены тома, следующая команда запуска создаст БД заново |
| Демо-данные | `docker compose exec core /app/core seed-demo` | создана «Кафе «Пример» (тестовые данные)» |
| Токен проверяющего | `docker compose exec core /app/core review-token issue --login reviewer_editor --role editor --ttl 720h` | в выводе строка `vvs_…` (показывается один раз) |
| Мониторинг | `docker compose --profile monitoring up -d prometheus` | `http://127.0.0.1:9091` |

Простыми словами: «том» — это папка Docker, где PostgreSQL хранит данные; `down` её не трогает, а `down -v` удаляет.

## 2. Развёртывание стенда (prod)

1. Сервер: Ubuntu 24.04, 2 vCPU, 4 ГБ, 40 ГБ SSD, публичный IPv4. Установить Docker Engine и плагин Compose по инструкции docs.docker.com для Ubuntu.
2. Сеть: открыть входящие 22 (SSH только по ключу), 80 и 443; остальное закрыть (`ufw default deny incoming; ufw allow 22,80,443/tcp; ufw enable`).
3. DNS: A-запись домена на IP сервера; проверить `dig +short <домен>`.
4. `git clone` репозитория в `/opt/vovremya`, перейти в каталог.
5. Сертификаты Минцифры: скачать корневой и выпускающий сертификаты по ссылкам с https://www.gosuslugi.ru/crt в `deploy/ca/russian_trusted_root_ca.pem` и `deploy/ca/russian_trusted_sub_ca.pem` (формат PEM); выполнить `openssl x509 -noout -subject -fingerprint -sha256 -in <файл>` и сверить SHA-256 с опубликованным на портале; записать отпечатки в таблицу раздела 6 этого документа и закоммитить файлы.
6. Токен бота положить в файл `/root/max_token` (права 600).
7. `python3 scripts/generate_prod_env.py --domain <домен> --acme-email <почта> --token-file /root/max_token` — создаст `.env` (права 600).
8. `docker compose up -d --build`; через 1–2 минуты `curl -I https://<домен>/` → `200`, `curl -s https://<домен>/api/v1/me` → 401 JSON.
9. `docker compose logs bot | grep -E "bot profile loaded|webhook subscription ensured"` — обе строки есть.
10. Привязать URL мини-приложения `https://<домен>/` к боту (spec §13), пройти AC-MAX-01…AC-MAX-12.

Простыми словами: Caddy сам получает бесплатный сертификат HTTPS, если домен указывает на сервер и порт 80 открыт; вручную с сертификатом сайта делать ничего не нужно.

## 3. Обновление

`git pull && docker compose up -d --build` — пересобираются изменённые образы, миграции выполняются автоматически перед стартом сервисов. Простой — секунды на пересоздание контейнера. Откат: `git checkout <предыдущий тег> && docker compose up -d --build`; миграции вниз не выполняются автоматически — изменения схемы в рамках хакатона только добавляющие.

## 4. Резервное копирование и восстановление

Ежедневно в 03:00 (cron root):

```sh
0 3 * * * cd /opt/vovremya && docker compose exec -T postgres pg_dump -U vovremya_admin -Fc vovremya > /var/backups/vovremya-$(date +\%F).dump && find /var/backups -name 'vovremya-*.dump' -mtime +7 -delete
```

Копия вне хоста: `scp` последнего дампа на ноутбук разработчика B ежедневно в плане INT-03. Восстановление на чистом хосте: развернуть по разделу 2 до шага 7, затем `docker compose up -d postgres`, `docker compose exec -T postgres pg_restore -U vovremya_admin -d vovremya --clean --if-exists < дамп`, затем `docker compose up -d`.

## 5. Типовые проблемы

| Симптом | Причина | Действие |
|---|---|---|
| `core-migrate` завершился с ошибкой | БД недоступна или конфликт миграции | `docker compose logs core-migrate`; исправить миграцию, повторить `up` |
| edge не получает сертификат | DNS не указывает на сервер или закрыт порт 80 | `dig`, `ufw status`, `docker compose logs edge` |
| Мини-приложение не открывается в веб-версии MAX | неверный `frame-ancestors` | в консоли браузера ошибка CSP → исправить `EDGE_FRAME_ANCESTORS` в `.env`, `docker compose up -d edge` |
| Все входы `LAUNCH_DATA_INVALID` | производный ключ не от того токена | пересчитать `scripts/derive_webapp_secret.py`, перезапустить core |
| Все входы `LAUNCH_DATA_EXPIRED` | часы сервера неверны | `timedatectl set-ntp true` |
| Напоминания не приходят | bot не подписан или 401 | логи bot: `max_401` — неверный токен; `subscription` — проверить домен и сертификат |
| Сервис `unhealthy` | нет связи с БД | `docker compose ps`, `docker compose logs postgres` |

## 6. Отпечатки сертификатов Минцифры

| Файл | SHA-256 (заполняется в INF-02 по данным gosuslugi.ru/crt) | Дата сверки |
|---|---|---|
| `deploy/ca/russian_trusted_root_ca.pem` | записывается исполнителем INF-02 | записывается исполнителем INF-02 |
| `deploy/ca/russian_trusted_sub_ca.pem` | записывается исполнителем INF-02 | записывается исполнителем INF-02 |
