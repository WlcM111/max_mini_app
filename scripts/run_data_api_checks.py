#!/usr/bin/env python3
"""Выполняет проверки из DATA-API.yaml. Требует PyYAML.
Токены: VV_TOKEN_EDITOR, VV_TOKEN_VIEWER; базовый адрес можно переопределить VV_BASE_URL."""
import json, os, sys, urllib.error, urllib.parse, urllib.request
import yaml

cfg = yaml.safe_load(open("DATA-API.yaml", encoding="utf-8"))
base = os.environ.get("VV_BASE_URL", cfg["base_url"]).rstrip("/")
fx = cfg.get("fixtures", {})


def has(obj, dotted):
    head, _, rest = dotted.partition(".")
    if head.endswith("[]"):
        arr = obj.get(head[:-2]) if isinstance(obj, dict) else None
        if not isinstance(arr, list):
            return False
        return all(has(x, rest) for x in arr) if rest else True
    if not isinstance(obj, dict) or head not in obj:
        return False
    return has(obj[head], rest) if rest else True


failed = 0
for c in cfg["checks"]:
    p = c.get("params") or {}
    path = c["path"].format(**fx)
    q = p.get("query") or {}
    url = base + path + ("?" + urllib.parse.urlencode(q) if q else "")
    body = p.get("body")
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=c["method"])
    for k, v in (p.get("headers") or {}).items():
        req.add_header(k, v)
    if c["role"] != "anonymous":
        req.add_header("Authorization", "Bearer " + os.environ.get("VV_TOKEN_" + c["role"].upper(), ""))
    try:
        resp = urllib.request.urlopen(req, timeout=15)
        status, ctype, raw = resp.status, resp.headers.get("Content-Type", ""), resp.read()
    except urllib.error.HTTPError as e:
        status, ctype, raw = e.code, e.headers.get("Content-Type", ""), e.read()
    ok = status in c["expected_status"]
    exp = c.get("response") or {}
    if ok and exp.get("content_type"):
        ok = ctype.split(";")[0].strip() == exp["content_type"]
    if ok and exp.get("required_fields"):
        try:
            doc = json.loads(raw or b"null")
            ok = all(has(doc, f) for f in exp["required_fields"])
        except ValueError:
            ok = False
    failed += 0 if ok else 1
    print(f"{'PASS' if ok else 'FAIL'} {c['id']} {c['method']} {path} -> {status} {ctype}")
sys.exit(1 if failed else 0)
