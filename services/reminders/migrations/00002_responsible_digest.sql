-- +goose Up
-- Ответственный участник документа: напоминания уходят лично ему (личные напоминания).
ALTER TABLE reminders.documents ADD COLUMN responsible_account_id uuid;

-- +goose Down
ALTER TABLE reminders.documents DROP COLUMN responsible_account_id;
