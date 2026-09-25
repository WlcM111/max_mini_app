# Проверяемые примеры

Версия 1.0.0 · 20.09.2026. Все примеры выполняются на локальном запуске (`docker compose up -d --build`) из корня репозитория; нужен Python 3 и curl.

## 1. Сессия и первые запросы

```sh
BODY=$(python3 scripts/sign_initdata.py --user-id 1001 --json)
curl -s -X POST http://localhost:8080/api/v1/sessions -H 'Content-Type: application/json' -d "$BODY"
# ожидается 201: {"token":"vvs_…","expires_at":"…","account":{"id":"…","first_name":"Тест",…},"start":{"kind":"none"}}
TOKEN=vvs_...   # значение token из ответа
curl -s http://localhost:8080/api/v1/me -H "Authorization: Bearer $TOKEN"
# ожидается 200: memberships = [], reminders_channel.state = "unknown", limits.max_documents_per_organization = 500
```

## 2. Организация и документы

```sh
ORG=0e3a9f5c-8b1d-4c2e-9a7f-1d2c3b4a5e61
curl -s -X POST http://localhost:8080/api/v1/organizations -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"id":"'$ORG'","name":"Кафе Ромашка (тестовые данные)","business_category_code":"food_service","region_code":"RU-SPE","timezone":"Europe/Moscow","feature_codes":["has_premises","has_employees","uses_kkt"]}'
# 201; повтор той же команды → 200 с той же организацией
curl -s http://localhost:8080/api/v1/organizations/$ORG/suggestions -H "Authorization: Bearer $TOKEN"
# 200: среди items есть qualified_esignature, kkt_fiscal_drive, employee_medical_exam, pest_control_contract
curl -s -X POST http://localhost:8080/api/v1/organizations/$ORG/documents/batch -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"items":[{"id":"5b1f7e2a-3c4d-4e5f-8a6b-7c8d9e0f1a21","document_type_code":"kkt_fiscal_drive","title":"Ключ ФН кассы №1","valid_until":"2026-10-05"}]}'
# 201: items[0].status = "expiring", reminder_offsets_days = [30,14,3]
curl -s "http://localhost:8080/api/v1/organizations/$ORG/documents?status=expiring" -H "Authorization: Bearer $TOKEN"
```

## 3. Ошибки

```sh
# Подделка: тест-вектор TV-2 из docs/max/test-vectors.md
curl -s -X POST http://localhost:8080/api/v1/sessions -H 'Content-Type: application/json' -d '{"init_data":"<строка TV-2>"}'
# 401: code = "LAUNCH_DATA_INVALID"
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/api/v1/me
# 401
```

## 4. Webhook и очередь бота

```sh
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:8080/max/webhook -d '{}'
# 401: нет секрета
curl -s -X POST http://localhost:8080/max/webhook -H 'X-Max-Bot-Api-Secret: devonly-webhook-secret' -H 'Content-Type: application/json' \
  -d '{"update_type":"bot_started","timestamp":1789900000000,"chat_id":5001,"user":{"user_id":1001,"first_name":"Тест"}}'
# 200 {}; в stub-буфере появилось приветствие:
docker compose exec bot wget -qO- http://127.0.0.1:8081/debug/stub/messages
docker run --rm --network vovremya_edge fullstorydev/grpcurl -plaintext \
  -d '{"idempotency_key":"test:manual:0001","kind":"NOTIFICATION_KIND_REMINDER","recipient_max_user_id":1001,"text":"Проверка","not_after":"2026-12-31T00:00:00Z"}' \
  bot:9090 vovremya.bot.v1.MessagingService/EnqueueNotification
# {"notificationId":"…","status":"NOTIFICATION_STATUS_QUEUED"}; повтор → "duplicate": true
```

## 5. Мини-приложение в браузере (имитация MAX)

Открыть `http://localhost:8080/?mockUser=1001` — пройдёт онбординг от имени пользователя 1001; `?mockUser=1002&startapp=inv_<токен>` — принятие приглашения вторым пользователем; `?startapp=doc_<uuid>` — открытие карточки как из напоминания.
