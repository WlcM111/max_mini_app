# Тест-векторы проверки данных запуска MAX

Версия 1.0.0 · 20.09.2026 · сгенерировано `scripts/sign_initdata.py`. Все векторы синтетические, токен `devonly-local-bot-token` не является рабочим.

Алгоритм — [спецификация, раздел 4](max-integration-spec.md#4-проверка-данных-запуска-на-backend).

| Параметр | Значение |
|---|---|
| Токен бота (dev) | `devonly-local-bot-token` |
| `CORE_MAX_WEBAPP_SECRET_HEX` | `e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663` |
| Момент проверки для TV-1 (`now`) | 1789900600 (auth_date + 10 минут) |

## TV-1: корректные данные — ожидается успех

```text
auth_date=1789900000&chat=%7B%22id%22%3A1001%2C%22type%22%3A%22DIALOG%22%7D&query_id=7a1c5e1e-2f63-4f3a-9d2b-0c8e5b1a4f10&user=%7B%22id%22%3A1001%2C%22first_name%22%3A%22%D0%A2%D0%B5%D1%81%D1%82%22%2C%22last_name%22%3Anull%2C%22username%22%3Anull%2C%22language_code%22%3A%22ru%22%2C%22photo_url%22%3Anull%7D&start_param=doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02&hash=f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc
```

Строка `launch_params` (разделитель — символ перевода строки 0x0A):

```text
auth_date=1789900000
chat={"id":1001,"type":"DIALOG"}
query_id=7a1c5e1e-2f63-4f3a-9d2b-0c8e5b1a4f10
start_param=doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02
user={"id":1001,"first_name":"Тест","last_name":null,"username":null,"language_code":"ru","photo_url":null}
```

Ожидаемый `hash`: `f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc`. Ожидаемый результат: user.id = 1001, start = {kind: document, document_id: 3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02}.

## TV-2: изменён user.id (1001 → 1002) при прежнем hash — ожидается `LAUNCH_DATA_INVALID`

```text
auth_date=1789900000&chat=%7B%22id%22%3A1001%2C%22type%22%3A%22DIALOG%22%7D&query_id=7a1c5e1e-2f63-4f3a-9d2b-0c8e5b1a4f10&user=%7B%22id%22%3A1002%2C%22first_name%22%3A%22%D0%A2%D0%B5%D1%81%D1%82%22%2C%22last_name%22%3Anull%2C%22username%22%3Anull%2C%22language_code%22%3A%22ru%22%2C%22photo_url%22%3Anull%7D&start_param=doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02&hash=f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc
```

## TV-3: параметр hash повторён дважды — ожидается `LAUNCH_DATA_INVALID`

```text
auth_date=1789900000&chat=%7B%22id%22%3A1001%2C%22type%22%3A%22DIALOG%22%7D&query_id=7a1c5e1e-2f63-4f3a-9d2b-0c8e5b1a4f10&user=%7B%22id%22%3A1001%2C%22first_name%22%3A%22%D0%A2%D0%B5%D1%81%D1%82%22%2C%22last_name%22%3Anull%2C%22username%22%3Anull%2C%22language_code%22%3A%22ru%22%2C%22photo_url%22%3Anull%7D&start_param=doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02&hash=f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc&hash=f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc
```

## TV-4: TV-1 при `now` = 1789903601 (auth_date + 3601 с) — ожидается `LAUNCH_DATA_EXPIRED`

## TV-5: TV-1 при `now` = 1789899939 (auth_date − 61 с, больше допустимого сдвига 60 с) — ожидается `LAUNCH_DATA_INVALID`

## TV-6: пустая строка, строка без `hash`, строка длиннее 4096 символов, `hash` не из 64 hex-символов — ожидается `LAUNCH_DATA_INVALID` (для длины > 4096 — `VALIDATION_FAILED`)

