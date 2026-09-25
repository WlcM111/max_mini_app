#!/usr/bin/env python3
"""Печатает hex(HMAC_SHA256("WebAppData", token)) — ключ проверки initData для core.
Токен читается из файла, чтобы не попадал в историю shell."""
import argparse
import hashlib
import hmac
import pathlib

p = argparse.ArgumentParser()
p.add_argument("--token-file", required=True)
token = pathlib.Path(p.parse_args().token_file).read_text(encoding="utf-8").strip()
print(hmac.new(b"WebAppData", token.encode(), hashlib.sha256).hexdigest())
