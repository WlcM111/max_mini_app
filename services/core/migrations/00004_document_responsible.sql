-- +goose Up
-- Ответственный участник документа (публичный UUID аккаунта): личные напоминания и фильтр «Мои документы».
ALTER TABLE core.documents ADD COLUMN responsible_account_id uuid;
CREATE INDEX documents_responsible_idx ON core.documents (organization_id, responsible_account_id)
    WHERE responsible_account_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS core.documents_responsible_idx;
ALTER TABLE core.documents DROP COLUMN responsible_account_id;
