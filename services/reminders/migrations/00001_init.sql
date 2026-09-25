-- reminders schema v1. Выполняется ролью reminders_migrator (владелец схемы reminders).
-- Схема и роли создаются deploy/postgres/init/01-init.sh.
-- Все таблицы принадлежат reminders-service; внешних ключей к схемам core и bot нет
-- (ADR-020: владение данными), связь с core — только по значениям идентификаторов.

-- +goose Up

-- Журнал принятых событий core: дедупликация доставки at-least-once по event_id.
CREATE TABLE reminders.inbox_events (
    event_id          uuid PRIMARY KEY,
    event_type        text NOT NULL CHECK (char_length(event_type) BETWEEN 1 AND 64),
    aggregate_type    text NOT NULL CHECK (aggregate_type IN ('organization', 'membership', 'document', 'account')),
    aggregate_id      text NOT NULL CHECK (char_length(aggregate_id) BETWEEN 1 AND 100),
    aggregate_version bigint NOT NULL CHECK (aggregate_version >= 0),
    schema_version    integer NOT NULL CHECK (schema_version >= 1),
    source_service    text NOT NULL CHECK (char_length(source_service) BETWEEN 1 AND 32),
    snapshot          boolean NOT NULL DEFAULT false,
    outcome           text NOT NULL CHECK (outcome IN ('applied', 'stale', 'rejected')),
    occurred_at       timestamptz NOT NULL,
    received_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inbox_events_received_idx ON reminders.inbox_events (received_at);
CREATE INDEX inbox_events_aggregate_idx ON reminders.inbox_events (aggregate_type, aggregate_id, aggregate_version DESC);

-- Локальная проекция организации: часовой пояс и название для текста напоминания.
CREATE TABLE reminders.organizations (
    organization_id uuid PRIMARY KEY,
    name            text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    timezone        text NOT NULL CHECK (char_length(timezone) BETWEEN 1 AND 64),
    version         bigint NOT NULL CHECK (version >= 0),
    deleted         boolean NOT NULL DEFAULT false,
    applied_at      timestamptz NOT NULL DEFAULT now()
);

-- Локальная проекция участия: получатели напоминаний и их настройки.
CREATE TABLE reminders.members (
    organization_id      uuid NOT NULL,
    account_id           uuid NOT NULL,
    account_kind         text NOT NULL CHECK (account_kind IN ('max', 'review')),
    max_user_id          bigint NOT NULL DEFAULT 0 CHECK (max_user_id >= 0),
    notify_enabled       boolean NOT NULL,
    notify_local_minutes smallint NOT NULL CHECK (notify_local_minutes BETWEEN 0 AND 1439),
    version              bigint NOT NULL CHECK (version >= 0),
    removed              boolean NOT NULL DEFAULT false,
    applied_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, account_id),
    CHECK ((account_kind = 'max' AND max_user_id > 0) OR (account_kind = 'review' AND max_user_id = 0))
);
CREATE INDEX members_account_idx ON reminders.members (account_id);

-- Локальная проекция документа с текущим периодом действия.
CREATE TABLE reminders.documents (
    document_id     uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    title           text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    period_id       uuid,
    valid_from      date,
    valid_until     date,
    version         bigint NOT NULL CHECK (version >= 0),
    deleted         boolean NOT NULL DEFAULT false,
    applied_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_until >= valid_from)
);
CREATE INDEX documents_org_idx ON reminders.documents (organization_id);

-- Отступы напоминаний документа (множество, вынесено в отдельную таблицу: 1НФ).
CREATE TABLE reminders.document_offsets (
    document_id uuid NOT NULL REFERENCES reminders.documents (document_id) ON DELETE CASCADE,
    days_before smallint NOT NULL CHECK (days_before BETWEEN 0 AND 365),
    PRIMARY KEY (document_id, days_before)
);

-- План напоминаний: одна строка на (период, получатель, отступ).
CREATE TABLE reminders.reminders (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id         uuid NOT NULL REFERENCES reminders.documents (document_id) ON DELETE CASCADE,
    organization_id     uuid NOT NULL,
    period_id           uuid NOT NULL,
    account_id          uuid NOT NULL,
    days_before         smallint NOT NULL CHECK (days_before BETWEEN 0 AND 365),
    due_at              timestamptz NOT NULL,
    status              text NOT NULL CHECK (status IN ('planned', 'handed_off', 'cancelled', 'skipped')),
    attempts            smallint NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at     timestamptz NOT NULL,
    last_error_code     text CHECK (char_length(last_error_code) <= 64),
    bot_notification_id uuid,
    handed_off_at       timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (period_id, account_id, days_before),
    CHECK ((status = 'handed_off') = (handed_off_at IS NOT NULL))
);
CREATE INDEX reminders_due_idx ON reminders.reminders (next_attempt_at) WHERE status = 'planned';
CREATE INDEX reminders_doc_idx ON reminders.reminders (document_id);
CREATE INDEX reminders_account_planned_idx ON reminders.reminders (account_id, document_id) WHERE status = 'planned';
CREATE INDEX reminders_org_idx ON reminders.reminders (organization_id) WHERE status = 'planned';
CREATE INDEX reminders_updated_idx ON reminders.reminders (updated_at) WHERE status <> 'planned';

-- Роль приложения не должна изменять историю миграций (исправление находки 7
-- прежнего архитектурного анализа). Роли может не быть в автономной тестовой среде.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'reminders_app') THEN
        EXECUTE 'REVOKE ALL ON reminders.goose_db_version FROM reminders_app';
        BEGIN
            EXECUTE 'REVOKE ALL ON SEQUENCE reminders.goose_db_version_id_seq FROM reminders_app';
        EXCEPTION WHEN undefined_table THEN
            NULL;
        END;
        EXECUTE 'GRANT SELECT ON reminders.goose_db_version TO reminders_app';
    END IF;
END$$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE reminders.reminders;
DROP TABLE reminders.document_offsets;
DROP TABLE reminders.documents;
DROP TABLE reminders.members;
DROP TABLE reminders.organizations;
DROP TABLE reminders.inbox_events;
