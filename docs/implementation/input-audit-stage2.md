# Аудит входных материалов этапа 2

Дата: 23.09.2026. Аудит первого этапа: [input-audit.md](input-audit.md).

## 1. Состав вложений

| Файл | SHA-256 | Формат | Способ изучения | Охват | Статус |
|---|---|---|---|---|---|
| vovremya-architecture-2_0_0.zip | `042dc698…3b5d708f` | zip, 255 записей / 181 файл | распаковка и чтение | 100 % состава, полное чтение материалов bot | изучен |
| Эффективныи__бизнес__1_.pdf | `28be9b3d…0608e8ca` | pdf, 22 страницы | постраничное чтение | 100 % | изучен |

В каталоге загрузок присутствуют также материалы предыдущих этапов
(`ARCHITECTURE_TZ.md`, `SHA256SUMS.txt`, `handoff-*.zip`,
`vovremya-architecture-tz-v1_0_0.zip`); они изучены на первом этапе и повторно
сверены по хешам.

## 2. Проверки архива

| Проверка | Результат |
|---|---|
| Целостность (`testzip`) | ошибок нет |
| Символические ссылки, пути с `..`, абсолютные пути | не найдены |
| Совпадение с рабочим деревом первого этапа | побайтно идентичен |
| Секреты в архиве | только значения с префиксом `devonly` |

## 3. Изученные материалы по реализуемому сервису

| Документ | Охват |
|---|---|
| docs/handoffs/bot-service.md (24 раздела) и bot-service-v2.md | полностью |
| api/proto/vovremya/bot/v1/messaging.proto | полностью |
| services/bot/migrations/00001_init.sql | полностью |
| docs/contracts/grpc-contract.md | полностью |
| docs/max/max-integration-spec.md §1–16 | полностью |
| docs/max/max-platform-facts.md (F-01…F-55, X-01, X-02, U-01…U-09) | полностью |
| docs/architecture/long-operations.md (D8-2, D8-4), sequences.md | полностью |
| docs/operations/configuration.md, compose-and-runbook.md | полностью |
| docs/plan/team-plan.md, team-plan-v2.md | полностью |
| docs/implementation/FIRST_SERVICE_REPORT.md | полностью |
| docs/requirements/requirements-registry.md, traceability-matrix.md, docs/testing/acceptance.md | разделы bot и интеграции |
| deploy/postgres/init/01-init.sh, deploy/edge/Caddyfile, Makefile, compose.yaml | полностью |
| PDF кейса «Эффективный бизнес» | полностью, включая критерии оценки |

## 4. Расхождения, найденные при изучении

| № | Материал | Расхождение | Действие |
|---|---|---|---|
| 1 | services/bot/migrations/00001_init.sql | CHECK с `{0,512}` невалиден в PostgreSQL (предел счётчика повторов — 255) | исправлено эквивалентным условием, дефект D-3 |
| 2 | services/reminders/internal/config/config.go | таймаут вызова bot 5 с против 2 с в контракте | исправлено, дефект A-1 |
| 3 | internal/platform/metrics/metrics.go | предметные метрики reminders в общем каркасе | вынесены в сервис, дефект B-1 |
| 4 | compose.yaml, профиль `test` | не заданы пароли ролей reminders для init-скрипта | добавлены, дефект D-6 |
| 5 | Makefile | генерация sqlc для bot при рукописном SQL | цель `gen-sql` оставлена только для core |
| 6 | docs/operations/repository-tree.md | планируемая структура bot не совпадает с принятой на первом этапе раскладкой слоёв | дерево обновлено по факту |
