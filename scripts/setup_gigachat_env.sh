#!/usr/bin/env bash
# Подключение языкового ассистента GigaChat (ADR-032).
# Скрипт спрашивает ключ авторизации, записывает его в .env рядом с остальными
# параметрами и по желанию проверяет ключ запросом токена.
# Ключ вводится скрыто и не попадает ни в историю shell, ни в репозиторий.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
ENV_FILE=".env"

if [ ! -f "$ENV_FILE" ]; then
  echo "Файл .env не найден. Создайте его (cp .env.example .env) и повторите." >&2
  exit 1
fi

# set_var заменяет строку переменной в .env либо добавляет её в конец файла.
set_var() {
  local name="$1" value="$2"
  if grep -qE "^${name}=" "$ENV_FILE"; then
    python3 - "$ENV_FILE" "$name" "$value" <<'PY'
import sys
path, name, value = sys.argv[1], sys.argv[2], sys.argv[3]
lines = open(path, encoding="utf-8").read().splitlines(keepends=True)
out = []
for line in lines:
    if line.startswith(name + "="):
        out.append(f"{name}={value}\n")
    else:
        out.append(line)
open(path, "w", encoding="utf-8").writelines(out)
PY
  else
    printf '%s=%s\n' "$name" "$value" >> "$ENV_FILE"
  fi
}

echo "=== Подключение GigaChat ==="
echo "Ключ авторизации берётся в личном кабинете Studio:"
echo "  проект GigaChat API → Настройки API → Получить ключ → поле Authorization Key."
echo "Это строка Base64 от 'Client ID:Client Secret'. Показывается один раз."
echo

AUTH_KEY=""
while [ -z "$AUTH_KEY" ]; do
  read -r -s -p "Ключ авторизации GigaChat: " AUTH_KEY
  echo
  AUTH_KEY="$(printf '%s' "$AUTH_KEY" | tr -d '[:space:]')"
  if [ -z "$AUTH_KEY" ]; then
    echo "  пустое значение, повторите" >&2
    continue
  fi
  if ! printf '%s' "$AUTH_KEY" | grep -qE '^[A-Za-z0-9+/=_-]{20,}$'; then
    echo "  это не похоже на ключ Base64, повторите" >&2
    AUTH_KEY=""
    continue
  fi
  if ! printf '%s' "$AUTH_KEY" | base64 -d >/dev/null 2>&1; then
    echo "  строка не декодируется как Base64, повторите" >&2
    AUTH_KEY=""
  fi
done

read -r -p "Область доступа [GIGACHAT_API_PERS]: " SCOPE
SCOPE="${SCOPE:-GIGACHAT_API_PERS}"
case "$SCOPE" in
  GIGACHAT_API_PERS|GIGACHAT_API_B2B|GIGACHAT_API_CORP) ;;
  *) echo "Недопустимая область доступа: $SCOPE" >&2; exit 1 ;;
esac

read -r -p "Модель [GigaChat-Pro]: " MODEL
MODEL="${MODEL:-GigaChat-Pro}"

read -r -p "Суточный бюджет токенов [200000]: " BUDGET
BUDGET="${BUDGET:-200000}"
if ! printf '%s' "$BUDGET" | grep -qE '^[0-9]+$'; then
  echo "Бюджет должен быть целым числом" >&2
  exit 1
fi

set_var CORE_GIGACHAT_AUTH_KEY "$AUTH_KEY"
set_var CORE_GIGACHAT_SCOPE "$SCOPE"
set_var CORE_GIGACHAT_MODEL "$MODEL"
set_var CORE_GIGACHAT_DAILY_TOKEN_BUDGET "$BUDGET"
set_var CORE_GIGACHAT_BASE_URL "${CORE_GIGACHAT_BASE_URL:-https://gigachat.devices.sberbank.ru/api/v1}"
set_var CORE_GIGACHAT_OAUTH_URL "${CORE_GIGACHAT_OAUTH_URL:-https://ngw.devices.sberbank.ru:9443/api/v2/oauth}"
set_var CORE_GIGACHAT_CA_FILE "${CORE_GIGACHAT_CA_FILE:-/etc/vovremya/ca/russian_trusted_ca_bundle.pem}"
set_var CORE_GIGACHAT_TIMEOUT "${CORE_GIGACHAT_TIMEOUT:-8s}"
set_var CORE_GIGACHAT_MAX_INPUT_CHARS "${CORE_GIGACHAT_MAX_INPUT_CHARS:-2000}"
set_var CORE_RATE_ASSISTANT_PER_MIN "${CORE_RATE_ASSISTANT_PER_MIN:-10}"
chmod 600 "$ENV_FILE"

echo
echo "Записано в .env:"
grep -E '^CORE_GIGACHAT_(SCOPE|MODEL|BASE_URL|OAUTH_URL|TIMEOUT|MAX_INPUT_CHARS|DAILY_TOKEN_BUDGET)=|^CORE_RATE_ASSISTANT_PER_MIN=' "$ENV_FILE"
echo "CORE_GIGACHAT_AUTH_KEY=<скрыт, $(printf '%s' "$AUTH_KEY" | wc -c | tr -d ' ') символов>"

echo
read -r -p "Проверить ключ запросом токена сейчас? [Y/n]: " CHECK
if [ "${CHECK:-Y}" = "Y" ] || [ "${CHECK:-Y}" = "y" ] || [ -z "${CHECK:-}" ]; then
  BUNDLE="$(mktemp)"
  trap 'rm -f "$BUNDLE"' EXIT
  awk '{sub(/\r$/,""); print}' deploy/ca/russian_trusted_root_ca.pem deploy/ca/russian_trusted_sub_ca.pem > "$BUNDLE"
  OAUTH_URL="$(grep -E '^CORE_GIGACHAT_OAUTH_URL=' "$ENV_FILE" | cut -d= -f2-)"
  RQ_UID="$(python3 -c 'import uuid;print(uuid.uuid4())')"
  HTTP_CODE="$(curl -sS -o /tmp/gigachat-token.json -w '%{http_code}' --cacert "$BUNDLE" \
    -X POST "$OAUTH_URL" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -H 'Accept: application/json' \
    -H "RqUID: $RQ_UID" \
    -H "Authorization: Basic $AUTH_KEY" \
    --data-urlencode "scope=$SCOPE" || echo "000")"
  if [ "$HTTP_CODE" = "200" ]; then
    echo "Токен получен: ключ рабочий."
  else
    echo "Не удалось получить токен, код $HTTP_CODE. Ответ:" >&2
    head -c 400 /tmp/gigachat-token.json 2>/dev/null || true
    echo >&2
    echo "Проверьте ключ и область доступа; сервис при этом останется работоспособным без ассистента." >&2
  fi
  rm -f /tmp/gigachat-token.json
fi

echo
echo "Дальше:"
echo "  docker compose up -d --build core"
echo "  docker compose logs core --tail 20 | grep -i ассистент"
echo "  curl -sS -H \"Authorization: Bearer <token>\" \$PUBLIC_BASE_URL/api/v1/me | grep assistant_enabled"
