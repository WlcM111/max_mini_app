#!/usr/bin/env python3
"""Синтетические данные запуска MAX (initData), подписанные по алгоритму
https://dev.max.ru/docs/webapps/validation. Только для локальных тестов."""
import argparse
import hashlib
import hmac
import json
import time
import urllib.parse
import uuid


def sign(token: str, user_id: int, first_name: str, start_param: str | None,
         auth_date: int, query_id: str) -> str:
    user = json.dumps({"id": user_id, "first_name": first_name, "last_name": None,
                       "username": None, "language_code": "ru", "photo_url": None},
                      ensure_ascii=False, separators=(",", ":"))
    fields = {"auth_date": str(auth_date),
              "chat": json.dumps({"id": user_id, "type": "DIALOG"}, separators=(",", ":")),
              "query_id": query_id, "user": user}
    if start_param:
        fields["start_param"] = start_param
    secret = hmac.new(b"WebAppData", token.encode(), hashlib.sha256).digest()
    launch = "\n".join(f"{k}={fields[k]}" for k in sorted(fields))
    fields["hash"] = hmac.new(secret, launch.encode(), hashlib.sha256).hexdigest()
    return "&".join(f"{k}={urllib.parse.quote(v, safe='')}" for k, v in fields.items())


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--token", default="devonly-local-bot-token")
    p.add_argument("--user-id", type=int, default=1001)
    p.add_argument("--first-name", default="Тест")
    p.add_argument("--start-param")
    p.add_argument("--auth-date", type=int, default=int(time.time()))
    p.add_argument("--query-id", default=str(uuid.uuid4()))
    p.add_argument("--json", action="store_true", help="печатать тело POST /sessions")
    a = p.parse_args()
    init_data = sign(a.token, a.user_id, a.first_name, a.start_param, a.auth_date, a.query_id)
    print(json.dumps({"init_data": init_data, "platform": "web"}) if a.json else init_data)


if __name__ == "__main__":
    main()
