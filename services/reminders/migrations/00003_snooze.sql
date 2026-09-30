-- +goose Up
-- Отложенный повтор напоминания («Напомнить через неделю», ADR-036) хранится строкой плана
-- вида snooze: на неё распространяются все отмены и проверки перед отправкой.
ALTER TABLE reminders.reminders
    ADD COLUMN kind text NOT NULL DEFAULT 'offset' CHECK (kind IN ('offset', 'snooze')),
    ADD COLUMN snooze_day integer CHECK (snooze_day > 0);
ALTER TABLE reminders.reminders
    ADD CONSTRAINT reminders_snooze_day_matches_kind CHECK ((kind = 'snooze') = (snooze_day IS NOT NULL));
-- Уникальность обычных напоминаний сохраняется; повтор уникален по дню нажатия.
ALTER TABLE reminders.reminders DROP CONSTRAINT reminders_period_id_account_id_days_before_key;
CREATE UNIQUE INDEX reminders_offset_key ON reminders.reminders (period_id, account_id, days_before)
    WHERE kind = 'offset';
CREATE UNIQUE INDEX reminders_snooze_key ON reminders.reminders (period_id, account_id, snooze_day)
    WHERE kind = 'snooze';

-- +goose Down
DELETE FROM reminders.reminders WHERE kind = 'snooze';
DROP INDEX reminders.reminders_snooze_key;
DROP INDEX reminders.reminders_offset_key;
ALTER TABLE reminders.reminders
    ADD CONSTRAINT reminders_period_id_account_id_days_before_key UNIQUE (period_id, account_id, days_before);
ALTER TABLE reminders.reminders DROP CONSTRAINT reminders_snooze_day_matches_kind;
ALTER TABLE reminders.reminders DROP COLUMN snooze_day, DROP COLUMN kind;
