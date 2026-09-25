#!/usr/bin/env python3
"""Статическая проверка compose.yaml.

Заменяет `docker compose config` там, где Docker недоступен: разбирает файл
(включая якоря и слияния), проверяет структурные инварианты развёртывания
«Вовремя» и согласованность с .env.example. Не заменяет запуск контейнеров,
но ловит ошибки конфигурации до сборки.
"""
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover
    print("требуется PyYAML: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

ROOT = Path(__file__).resolve().parent.parent
errors: list[str] = []
checks = 0


def check(condition: bool, message: str) -> None:
    global checks
    checks += 1
    if not condition:
        errors.append(message)


def main() -> int:
    raw = (ROOT / "compose.yaml").read_text(encoding="utf-8")
    data = yaml.safe_load(raw)
    services = data.get("services", {})
    check(bool(services), "в compose.yaml нет сервисов")

    for name, svc in services.items():
        check("image" in svc or "build" in svc, f"{name}: нет ни image, ни build")

    # Приложения и их миграции.
    for app in ("bot", "reminders", "core"):
        if app not in services:
            continue
        svc = services[app]
        env = svc.get("environment", {})
        check(svc.get("command") == ["serve"], f"{app}: команда должна быть serve")
        check("healthcheck" in svc, f"{app}: нет healthcheck")
        check(svc.get("stop_grace_period") is not None, f"{app}: не задан stop_grace_period")
        check("ports" not in svc, f"{app}: публикует порты напрямую, наружу смотрит только edge")
        migrate = f"{app}-migrate"
        check(migrate in services, f"нет сервиса миграций {migrate}")
        depends = svc.get("depends_on", {})
        check(
            isinstance(depends, dict) and depends.get(migrate, {}).get("condition") == "service_completed_successfully",
            f"{app}: должен ждать успешного завершения {migrate}",
        )
        # Пароли ролей: приложение не должно получать учётные данные мигратора.
        for key, value in env.items():
            if key.endswith("_DATABASE_URL") and "MIGRATE" not in key:
                check("_migrator:" not in str(value), f"{app}: {key} использует роль мигратора")
        if migrate in services:
            menv = services[migrate].get("environment", {})
            check(
                any("_migrator:" in str(v) for k, v in menv.items() if k.endswith("_DATABASE_URL")),
                f"{migrate}: не задан DSN роли мигратора",
            )
            check(services[migrate].get("restart") == "no", f"{migrate}: restart должен быть \"no\"")

    # Сетевая привязка bot: адрес и псевдоним для gRPC из соседних сервисов.
    bot = services.get("bot", {})
    bot_networks = bot.get("networks", {})
    check("data" in bot_networks and "edge" in bot_networks, "bot: должен быть в сетях edge и data")
    data_net = bot_networks.get("data", {}) or {}
    check(bool(data_net.get("ipv4_address")), "bot: не задан фиксированный адрес в сети data")
    check("bot-grpc" in (data_net.get("aliases") or []), "bot: нет сетевого псевдонима bot-grpc")
    bot_env = bot.get("environment", {})
    for key in ("BOT_MODE", "BOT_DATABASE_URL", "BOT_GRPC_ADDR", "BOT_HTTP_ADDR", "BOT_ADMIN_ADDR",
                "BOT_WEBHOOK_SECRET", "BOT_MAX_API_BASE_URL"):
        check(key in bot_env, f"bot: не задана переменная {key}")
    grpc_addr = str(bot_env.get("BOT_GRPC_ADDR", ""))
    check(grpc_addr.endswith(":9090"), "bot: gRPC должен слушать порт 9090 (grpc-contract)")

    # reminders обращается к bot по адресу из той же сети.
    rem_env = services.get("reminders", {}).get("environment", {})
    target = str(rem_env.get("REMINDERS_BOT_GRPC_ADDR", ""))
    check(target.endswith(":9090"), "reminders: REMINDERS_BOT_GRPC_ADDR должен указывать на порт 9090")
    check(
        target.split(":")[0] in {"bot", "bot-grpc", data_net.get("ipv4_address")},
        f"reminders: адрес bot {target!r} не соответствует сервису bot",
    )

    # Инициализация PostgreSQL: скрипт монтируется только для чтения и получает
    # пароли всех шести ролей, иначе он прерывается (set -eu).
    init_vars = {
        m.group(1)
        for m in re.finditer(r"\$(PG_[A-Z_]+_PASSWORD)", (ROOT / "deploy/postgres/init/01-init.sh").read_text(encoding="utf-8"))
    }
    for pg in ("postgres", "postgres-test"):
        if pg not in services:
            continue
        env = services[pg].get("environment", {})
        for var in sorted(init_vars):
            check(var in env, f"{pg}: не задан {var}, init-скрипт прервётся")
        mounts = services[pg].get("volumes", [])
        check(
            any(str(v).endswith(":ro") and "docker-entrypoint-initdb.d" in str(v) for v in mounts),
            f"{pg}: init-скрипт должен монтироваться только для чтения",
        )

    # Подстановки окружения: либо значение по умолчанию, либо запись в .env.example.
    example = (ROOT / ".env.example").read_text(encoding="utf-8") if (ROOT / ".env.example").exists() else ""
    documented = {line.split("=", 1)[0].strip() for line in example.splitlines() if "=" in line and not line.startswith("#")}
    for match in re.finditer(r"\$\{([A-Z0-9_]+)(:-[^}]*)?\}", raw):
        var, default = match.group(1), match.group(2)
        check(default is not None or var in documented,
              f"переменная {var} не имеет значения по умолчанию и не описана в .env.example")

    print(f"проверок выполнено: {checks}")
    if errors:
        print("НАЙДЕНЫ ОШИБКИ:")
        for e in errors:
            print(f"  - {e}")
        return 1
    print("compose.yaml согласован")
    return 0


if __name__ == "__main__":
    sys.exit(main())
