#!/usr/bin/env python3
"""Создаёт .env для APP_ENV=prod со случайными паролями. Существующий .env не перезаписывает."""
import argparse
import hashlib
import hmac
import os
import pathlib
import secrets
import string
import sys

ALPHABET = string.ascii_letters + string.digits


def rnd(n: int) -> str:
    return "".join(secrets.choice(ALPHABET) for _ in range(n))


p = argparse.ArgumentParser()
p.add_argument("--domain", required=True, help="например app.vovremya.ru")
p.add_argument("--acme-email", required=True)
p.add_argument("--token-file", required=True, help="файл с токеном бота от организаторов")
p.add_argument("--version", default="1.0.0")
a = p.parse_args()
env = pathlib.Path(".env")
if env.exists():
    sys.exit(".env уже существует — удалите его осознанно или правьте вручную")
token = pathlib.Path(a.token_file).read_text(encoding="utf-8").strip()
derived = hmac.new(b"WebAppData", token.encode(), hashlib.sha256).hexdigest()
lines = {
    "APP_ENV": "prod", "APP_VERSION": a.version, "LOG_LEVEL": "info",
    "POSTGRES_PASSWORD": rnd(32), "PG_CORE_MIGRATOR_PASSWORD": rnd(32),
    "PG_CORE_APP_PASSWORD": rnd(32), "PG_BOT_MIGRATOR_PASSWORD": rnd(32),
    "PG_BOT_APP_PASSWORD": rnd(32),
    "BOT_MODE": "live", "MAX_BOT_TOKEN": token,
    "BOT_WEBHOOK_PUBLIC_URL": f"https://{a.domain}/max/webhook",
    "BOT_WEBHOOK_SECRET": rnd(64), "BOT_OPEN_APP_BUTTON_KIND": "link",
    "CORE_MAX_WEBAPP_SECRET_HEX": derived,
    "PUBLIC_BASE_URL": f"https://{a.domain}",
    "EDGE_SITE_ADDRESS": a.domain, "EDGE_HTTP_PORT": "80", "EDGE_HTTPS_PORT": "443",
    "ACME_EMAIL": a.acme_email, "VITE_MOCK_BRIDGE": "false",
}
env.write_text("".join(f"{k}={v}\n" for k, v in lines.items()), encoding="utf-8")
os.chmod(env, 0o600)
print(".env создан (права 600). Токен бота в git не добавлять.")
