#!/usr/bin/env python3
"""Выполняет проверки из DATA-API.yaml. Требует PyYAML.
Токены: VV_TOKEN_EDITOR, VV_TOKEN_VIEWER; базовый адрес можно переопределить VV_BASE_URL.
capture сохраняет значение из ответа для следующих проверок, signature сверяет первые байты файла.
Выполняются обязательные проверки (checks), затем дополнительные (additional_checks)."""
import json, os, re, sys, urllib.error, urllib.parse, urllib.request
import yaml

cfg = yaml.safe_load(open("DATA-API.yaml", encoding="utf-8"))
base = os.environ.get("VV_BASE_URL", cfg["base_url"]).rstrip("/")
fx = dict(cfg.get("fixtures", {}))


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
checks = [(c, "") for c in cfg["checks"]] + [(c, "доп. ") for c in cfg.get("additional_checks") or []]
for c, label in checks:
    p = c.get("params") or {}
    try:
        path = c["path"].format(**fx)
    except KeyError as missing:
        failed += 1
        print(f"FAIL {label}{c['id']}: нет значения {missing} из предыдущей проверки")
        continue
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
        resp = urllib.request.urlopen(req, timeout=60)
        status, ctype, raw = resp.status, resp.headers.get("Content-Type", ""), resp.read()
    except urllib.error.HTTPError as e:
        status, ctype, raw = e.code, e.headers.get("Content-Type", ""), e.read()
    ok = status in c["expected_status"]
    exp = c.get("response") or {}
    if ok and exp.get("content_type"):
        ok = ctype.split(";")[0].strip() == exp["content_type"]
    if ok and exp.get("signature"):
        ok = raw[: len(exp["signature"])] == exp["signature"].encode()
    doc = None
    if ctype.split(";")[0].strip().endswith("json"):
        try:
            doc = json.loads(raw or b"null")
        except ValueError:
            doc = None
    if ok and exp.get("required_fields"):
        ok = doc is not None and all(has(doc, f) for f in exp["required_fields"])
    for name, rule in (c.get("capture") or {}).items():
        found = re.search(rule["pattern"], str(doc.get(rule["field"], "")) if isinstance(doc, dict) else "")
        if ok and found:
            fx[name] = found.group(1)
        else:
            ok = False
    failed += 0 if ok else 1
    print(f"{'PASS' if ok else 'FAIL'} {label}{c['id']} {c['method']} {path} -> {status} {ctype}")
sys.exit(1 if failed else 0)
