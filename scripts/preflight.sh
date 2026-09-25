#!/usr/bin/env bash
# Проверка готовности репозитория к боевому запуску: дефекты D-16..D-23.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
fail=0
ok()  { printf '  OK   %s\n' "$1"; }
bad() { printf '  FAIL %s\n' "$1"; fail=1; }

echo "== Сборка образов =="
[ "$(grep -c '^COPY internal ./internal$' services/core/Dockerfile)" = 1 ] && ok "core: COPY internal (D-17)" || bad "core: нужна одна строка COPY internal ./internal"
[ "$(grep -c '^COPY internal ./internal$' services/bot/Dockerfile)"  = 1 ] && ok "bot: COPY internal (D-17)"  || bad "bot: нужна одна строка COPY internal ./internal"
[ "$(grep -c 'image: vovremya/.*-migrate:' compose.yaml)" = 3 ] && ok "отдельные теги образов миграций (D-18)" || bad "у сервисов *-migrate должны быть свои теги образов"

echo "== Запуск =="
[ "$(grep -c 'pg_isready -h 127.0.0.1' compose.yaml)" = 2 ] && ok "healthcheck PostgreSQL по TCP (D-19)" || bad "healthcheck должен использовать pg_isready -h 127.0.0.1"
[ -x deploy/postgres/init/01-init.sh ] && ok "init-скрипт исполняемый (D-20)" || bad "chmod +x deploy/postgres/init/01-init.sh"
[ "$(git ls-files -s deploy/postgres/init/01-init.sh | awk '{print $1}')" = 100755 ] && ok "режим 100755 в git (D-20)" || bad "git update-index --chmod=+x deploy/postgres/init/01-init.sh"
grep -q 'BOT_MODE: ${BOT_MODE' compose.yaml && ok "bot-migrate получает BOT_MODE (D-21)" || bad "в bot-migrate нет BOT_MODE"

echo "== Сертификаты канала MAX =="
certs=$(awk '{sub(/\r$/,""); print}' deploy/ca/*.pem 2>/dev/null | grep -c '^-----BEGIN CERTIFICATE-----$')
[ "$certs" -ge 2 ] && ok "в deploy/ca найдено сертификатов: $certs (D-22)" || bad "в deploy/ca меньше двух сертификатов: $certs"
openssl x509 -in deploy/ca/russian_trusted_root_ca.pem -noout -checkend 0 >/dev/null 2>&1 && ok "корневой сертификат действует" || bad "корневой сертификат просрочен или нечитаем"
grep -q 'sub(/\\r\$/' services/bot/Dockerfile && ok "бандл CA нормализуется в образе (D-23)" || bad "в Dockerfile бота склейка ломает PEM"

echo "== Зависимости Go =="
grep -q "3Dnd1cDaZlB68lziofO" go.sum && ok "go.sum с официального прокси (D-16)" || bad "go.sum содержит хеши локального зеркала: rm go.sum && go mod download all"

echo "== Секреты =="
git ls-files | grep -qE "(^|/)\.env$" && bad ".env попал в репозиторий" || ok ".env не в репозитории"
grep -q "^\.env$" .gitignore && ok ".env в .gitignore" || bad "добавьте .env в .gitignore"

echo "== Состав compose =="
python3 scripts/check_compose.py >/dev/null 2>&1 && ok "compose.yaml согласован" || bad "python3 scripts/check_compose.py не проходит"

echo
[ "$fail" = 0 ] && echo "ГОТОВО: можно пушить и разворачивать" || echo "ЕСТЬ ПРОБЛЕМЫ — см. FAIL выше"
exit "$fail"
