#!/usr/bin/env python3
"""Пересобирает ARCHIVE_MANIFEST.md и docs/implementation/archive-manifest.json по составу git
(отслеживаемые и новые неигнорируемые файлы). Содержимое берётся из индекса git, поэтому перед
запуском выполните git add -A. Запуск из любого каталога репозитория:
    python3 scripts/make_manifest.py"""
from __future__ import annotations

import datetime
import hashlib
import json
import pathlib
import posixpath
import subprocess
import sys

MD = "ARCHIVE_MANIFEST.md"
JSON = "docs/implementation/archive-manifest.json"
# tsconfig.tsbuildinfo — кэш инкрементальной сборки: меняется при каждом npm run build,
# поэтому в манифест не входит, иначе он устаревает сразу после сборки.
EXCLUDED = [MD, JSON, "frontend/node_modules", "frontend/dist", "frontend/tsconfig.tsbuildinfo"]


def git(root: pathlib.Path, *args: str, stdin: bytes | None = None) -> bytes:
    return subprocess.run(["git", "-C", str(root), *args], input=stdin, check=True, capture_output=True).stdout


def index_blobs(root: pathlib.Path, paths: list[str]) -> dict[str, bytes]:
    """Содержимое файлов из индекса одним вызовом git cat-file --batch."""
    out = git(root, "cat-file", "--batch", stdin="".join(f":{p}\n" for p in paths).encode())
    blobs, pos = {}, 0
    for p in paths:
        end = out.index(b"\n", pos)
        header = out[pos:end].split()
        if header[-1] == b"missing":
            raise SystemExit(f"нет в индексе: {p}")
        size = int(header[2])
        blobs[p] = out[end + 1:end + 1 + size]
        pos = end + 1 + size + 1
    return blobs


def main() -> int:
    root = pathlib.Path(subprocess.run(["git", "rev-parse", "--show-toplevel"], check=True,
                                       capture_output=True, text=True).stdout.strip())
    if git(root, "diff", "--name-only"):
        print("есть неиндексированные изменения: выполните git add -A и запустите снова", file=sys.stderr)
        return 1
    cached = [p for p in git(root, "ls-files", "-z").decode().split("\0") if p]
    new = [p for p in git(root, "ls-files", "-z", "--others", "--exclude-standard").decode().split("\0") if p]
    listed = sorted(p for p in set(cached) | set(new)
                    if p not in EXCLUDED and not any(p.startswith(e + "/") for e in EXCLUDED))
    blobs = index_blobs(root, [p for p in listed if p in set(cached)])
    files = []
    for p in listed:
        data = blobs[p] if p in blobs else (root / p).read_bytes()
        files.append({"path": p, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()})

    today = datetime.date.today()
    total = len(files) + 2
    size = sum(f["bytes"] for f in files)
    frontend = sum(1 for f in files if f["path"].startswith("frontend/"))
    dirs = {posixpath.dirname(f["path"]) for f in files} | {posixpath.dirname(JSON)}
    dirs.discard("")

    manifest = {
        "generated": today.isoformat(),
        "architecture_version": "2.0.0",
        "stage": "MAX Mini App (frontend)",
        "services_implemented": ["reminders-service", "bot-service", "core-service"],
        "frontend_implemented": True,
        "files_total": total,
        "files_listed": len(files),
        "bytes_listed": size,
        "frontend_files": frontend,
        "excluded": EXCLUDED,
        "files": files,
    }
    (root / JSON).write_text(json.dumps(manifest, ensure_ascii=False, indent=1), encoding="utf-8")

    lines = [
        "# Состав архива",
        "",
        f"Всего файлов в архиве: {total}. Каталогов: {len(dirs)}. Дата: {today:%d.%m.%Y}. Версия архитектуры: 2.0.0.",
        "",
        f"Три backend-микросервиса и MAX Mini App (каталог `frontend/`, {frontend} файлов), нормативная документация,",
        "контракты, миграции, тесты и конфигурация запуска.",
        "",
        "Каталоги `frontend/node_modules` и `frontend/dist` не входят: зависимости восстанавливаются",
        "командой `npm ci` по `frontend/package-lock.json`.",
        "",
        f"Ниже перечислены {len(files)} файлов суммарным размером {size} байт — состав git-репозитория.",
        "Два файла манифеста (`ARCHIVE_MANIFEST.md` и `docs/implementation/archive-manifest.json`) в перечень",
        "не входят: их содержимое зависит от самого перечня. Пересборка — `git add -A` и",
        "`python3 scripts/make_manifest.py`; архив для сдачи — `git archive` по тегу (docs/operations/submission.md).",
        "",
        "| Файл | Размер, байт | SHA-256 |",
        "|---|---|---|",
    ]
    lines += [f"| {f['path']} | {f['bytes']} | {f['sha256'][:16]}… |" for f in files]
    (root / MD).write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"{MD}: {len(files)} файлов, {size} байт")
    return 0


if __name__ == "__main__":
    sys.exit(main())
