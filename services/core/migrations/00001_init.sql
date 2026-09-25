-- core schema v1. Выполняется ролью core_migrator (владелец схемы core).
-- Схема core и роли создаются deploy/postgres/init/01-init.sh.

-- +goose Up
-- Справочники (модельные данные загружает 00002_catalog_seed.sql)
CREATE TABLE core.business_categories (
    code        text PRIMARY KEY CHECK (code ~ '^[a-z][a-z0-9_]{1,39}$'),
    title       text NOT NULL UNIQUE CHECK (char_length(title) BETWEEN 1 AND 100),
    sort_order  smallint NOT NULL
);

CREATE TABLE core.regions (
    code             text PRIMARY KEY CHECK (code ~ '^RU-[A-Z]{2,3}$' OR code = 'XX-OTHER'),
    title            text NOT NULL UNIQUE CHECK (char_length(title) BETWEEN 1 AND 100),
    default_timezone text NOT NULL CHECK (char_length(default_timezone) BETWEEN 3 AND 64),
    sort_order       smallint NOT NULL
);

CREATE TABLE core.features (
    code        text PRIMARY KEY CHECK (code ~ '^[a-z][a-z0-9_]{1,39}$'),
    question    text NOT NULL CHECK (char_length(question) BETWEEN 1 AND 200),
    hint        text CHECK (char_length(hint) <= 300),
    sort_order  smallint NOT NULL
);

CREATE TABLE core.catalog_sources (
    id          smallint PRIMARY KEY,
    title       text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    url         text NOT NULL CHECK (url ~ '^https://' AND char_length(url) <= 500),
    checked_on  date
);

CREATE TABLE core.document_types (
    code         text PRIMARY KEY CHECK (code ~ '^[a-z][a-z0-9_]{1,39}$'),
    title        text NOT NULL UNIQUE CHECK (char_length(title) BETWEEN 1 AND 200),
    description  text NOT NULL CHECK (char_length(description) BETWEEN 1 AND 500),
    data_status  text NOT NULL CHECK (data_status IN ('model', 'verified')),
    source_id    smallint REFERENCES core.catalog_sources (id),
    sort_order   smallint NOT NULL,
    CHECK (data_status = 'model' OR source_id IS NOT NULL)
);

CREATE TABLE core.document_type_default_offsets (
    document_type_code text NOT NULL REFERENCES core.document_types (code) ON DELETE CASCADE,
    days_before        smallint NOT NULL CHECK (days_before BETWEEN 0 AND 365),
    PRIMARY KEY (document_type_code, days_before)
);

CREATE TABLE core.document_type_renewal_steps (
    document_type_code text NOT NULL REFERENCES core.document_types (code) ON DELETE CASCADE,
    step_no            smallint NOT NULL CHECK (step_no BETWEEN 1 AND 20),
    step_text          text NOT NULL CHECK (char_length(step_text) BETWEEN 1 AND 300),
    PRIMARY KEY (document_type_code, step_no)
);

-- Правило применимости: тип документа подходит организации, если категория совпадает
-- (или NULL = любая) и у организации есть ВСЕ признаки правила.
CREATE TABLE core.applicability_rules (
    id                     smallint PRIMARY KEY,
    document_type_code     text NOT NULL REFERENCES core.document_types (code) ON DELETE CASCADE,
    business_category_code text REFERENCES core.business_categories (code)
);
CREATE INDEX applicability_rules_type_idx ON core.applicability_rules (document_type_code);

CREATE TABLE core.applicability_rule_features (
    rule_id      smallint NOT NULL REFERENCES core.applicability_rules (id) ON DELETE CASCADE,
    feature_code text NOT NULL REFERENCES core.features (code),
    PRIMARY KEY (rule_id, feature_code)
);

-- Пользователи и сессии
CREATE TABLE core.accounts (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id     uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    kind          text NOT NULL CHECK (kind IN ('max', 'review')),
    max_user_id   bigint UNIQUE,
    review_login  text UNIQUE CHECK (review_login ~ '^[a-z][a-z0-9_]{2,31}$'),
    first_name    text NOT NULL CHECK (char_length(first_name) BETWEEN 1 AND 128),
    last_name     text CHECK (char_length(last_name) <= 128),
    username      text CHECK (char_length(username) <= 64),
    language_code text CHECK (char_length(language_code) <= 16),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'max' AND max_user_id IS NOT NULL AND review_login IS NULL)
        OR (kind = 'review' AND max_user_id IS NULL AND review_login IS NOT NULL))
);

CREATE TABLE core.sessions (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash      bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    account_id      bigint NOT NULL REFERENCES core.accounts (id) ON DELETE CASCADE,
    source          text NOT NULL CHECK (source IN ('max_launch', 'review_cli')),
    launch_query_id text CHECK (char_length(launch_query_id) <= 64),
    platform        text CHECK (platform IN ('ios', 'android', 'desktop', 'web')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    revoked_at      timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX sessions_account_idx ON core.sessions (account_id);
CREATE INDEX sessions_expires_idx ON core.sessions (expires_at);

-- Организации и участники
CREATE TABLE core.organizations (
    id                     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id              uuid NOT NULL UNIQUE,
    name                   text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    business_category_code text NOT NULL REFERENCES core.business_categories (code),
    region_code            text NOT NULL REFERENCES core.regions (code),
    timezone               text NOT NULL CHECK (char_length(timezone) BETWEEN 3 AND 64),
    version                integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by             bigint REFERENCES core.accounts (id) ON DELETE SET NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.organization_features (
    organization_id bigint NOT NULL REFERENCES core.organizations (id) ON DELETE CASCADE,
    feature_code    text NOT NULL REFERENCES core.features (code),
    PRIMARY KEY (organization_id, feature_code)
);

CREATE TABLE core.memberships (
    organization_id   bigint NOT NULL REFERENCES core.organizations (id) ON DELETE CASCADE,
    account_id        bigint NOT NULL REFERENCES core.accounts (id) ON DELETE CASCADE,
    role              text NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    notify_enabled    boolean NOT NULL DEFAULT true,
    notify_local_time time NOT NULL DEFAULT '09:00' CHECK (EXTRACT(SECOND FROM notify_local_time) = 0),
    -- Версия участия (handoff core-service-v2 §2.4): увеличивается при изменении
    -- роли и настроек уведомлений, переносится в событие для reminders-service.
    version           integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    joined_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, account_id)
);
CREATE UNIQUE INDEX memberships_one_owner_idx ON core.memberships (organization_id) WHERE role = 'owner';
CREATE INDEX memberships_account_idx ON core.memberships (account_id);

CREATE TABLE core.invites (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id       uuid NOT NULL UNIQUE,
    organization_id bigint NOT NULL REFERENCES core.organizations (id) ON DELETE CASCADE,
    token_hash      bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    role            text NOT NULL CHECK (role IN ('editor', 'viewer')),
    created_by      bigint REFERENCES core.accounts (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    accepted_by     bigint REFERENCES core.accounts (id) ON DELETE SET NULL,
    accepted_at     timestamptz,
    revoked_at      timestamptz,
    CHECK (expires_at > created_at),
    CHECK (accepted_by IS NULL OR accepted_at IS NOT NULL),
    CHECK (NOT (accepted_at IS NOT NULL AND revoked_at IS NOT NULL))
);
CREATE INDEX invites_org_active_idx ON core.invites (organization_id)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- Документы и периоды действия
CREATE TABLE core.documents (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id          uuid NOT NULL UNIQUE,
    organization_id    bigint NOT NULL REFERENCES core.organizations (id) ON DELETE CASCADE,
    document_type_code text REFERENCES core.document_types (code),
    title              text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    number             text CHECK (char_length(number) <= 100),
    issuer             text CHECK (char_length(issuer) <= 200),
    responsible_label  text CHECK (char_length(responsible_label) <= 100),
    notes              text CHECK (char_length(notes) <= 2000),
    reference_url      text CHECK (reference_url IS NULL
                                   OR (reference_url ~ '^https://' AND char_length(reference_url) <= 1024)),
    version            integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by         bigint REFERENCES core.accounts (id) ON DELETE SET NULL,
    updated_by         bigint REFERENCES core.accounts (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX documents_org_idx ON core.documents (organization_id);

CREATE TABLE core.document_periods (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id   uuid NOT NULL UNIQUE,
    document_id bigint NOT NULL REFERENCES core.documents (id) ON DELETE CASCADE,
    valid_from  date,
    valid_until date,
    is_current  boolean NOT NULL,
    created_by  bigint REFERENCES core.accounts (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_until >= valid_from)
);
CREATE UNIQUE INDEX document_periods_one_current_idx ON core.document_periods (document_id) WHERE is_current;
CREATE INDEX document_periods_doc_created_idx ON core.document_periods (document_id, created_at DESC);

CREATE TABLE core.document_reminder_offsets (
    document_id bigint NOT NULL REFERENCES core.documents (id) ON DELETE CASCADE,
    days_before smallint NOT NULL CHECK (days_before BETWEEN 0 AND 365),
    PRIMARY KEY (document_id, days_before)
);

-- Исходящие события для reminders-service (архитектура 2.0.0, handoff core-service-v2 §2.1).
-- Строка пишется в одной транзакции с изменением предметных данных; отдельная
-- транзакция для события запрещена. Таблица core.reminders версии 1.0.0 удалена:
-- план напоминаний принадлежит reminders-service (ADR-017).
CREATE TABLE core.outbox_events (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id          uuid NOT NULL UNIQUE,
    event_type        text NOT NULL CHECK (event_type IN (
                          'organization_state', 'organization_deleted', 'membership_state',
                          'membership_removed', 'document_state', 'document_deleted', 'account_deleted')),
    aggregate_type    text NOT NULL CHECK (aggregate_type IN ('organization', 'membership', 'document', 'account')),
    aggregate_id      text NOT NULL CHECK (char_length(aggregate_id) BETWEEN 1 AND 100),
    aggregate_version bigint NOT NULL CHECK (aggregate_version >= 1),
    payload           jsonb NOT NULL,
    snapshot          boolean NOT NULL DEFAULT false,
    status            text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts          smallint NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at   timestamptz NOT NULL DEFAULT now(),
    last_error_code   text CHECK (char_length(last_error_code) <= 64),
    created_at        timestamptz NOT NULL DEFAULT now(),
    sent_at           timestamptz,
    CHECK ((status = 'sent') = (sent_at IS NOT NULL))
);
CREATE INDEX outbox_ready_idx ON core.outbox_events (next_attempt_at) WHERE status <> 'sent';
CREATE INDEX outbox_aggregate_idx ON core.outbox_events (aggregate_type, aggregate_id, aggregate_version);
CREATE INDEX outbox_created_idx ON core.outbox_events (created_at);

-- Одноразовые ссылки скачивания ICS
CREATE TABLE core.calendar_exports (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash      bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    organization_id bigint NOT NULL REFERENCES core.organizations (id) ON DELETE CASCADE,
    account_id      bigint NOT NULL REFERENCES core.accounts (id) ON DELETE CASCADE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    download_count  smallint NOT NULL DEFAULT 0 CHECK (download_count BETWEEN 0 AND 3),
    CHECK (expires_at > created_at)
);
CREATE INDEX calendar_exports_expires_idx ON core.calendar_exports (expires_at);

-- Журнал значимых действий (без ПДн, кроме ссылок на аккаунт)
CREATE TABLE core.audit_events (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at      timestamptz NOT NULL DEFAULT now(),
    account_id       bigint REFERENCES core.accounts (id) ON DELETE SET NULL,
    organization_id  bigint REFERENCES core.organizations (id) ON DELETE SET NULL,
    action           text NOT NULL CHECK (action IN (
                         'session.created', 'organization.created', 'organization.deleted',
                         'member.role_changed', 'member.removed', 'invite.created',
                         'invite.accepted', 'invite.revoked', 'document.deleted',
                         'export.created', 'review_token.issued', 'account.deleted')),
    target_public_id uuid
);
CREATE INDEX audit_events_org_idx ON core.audit_events (organization_id, occurred_at DESC);
CREATE INDEX audit_events_occurred_idx ON core.audit_events (occurred_at);

-- Справочники доступны сервису только на чтение.
-- Роль core_app создаёт deploy/postgres/init/01-init.sh; в изолированной тестовой
-- базе её может не быть, поэтому права снимаются условно (дефект D-7 этапа 3:
-- безусловный REVOKE прерывал применение миграций на чистой PostgreSQL).
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'core_app') THEN
        EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON
            core.business_categories, core.regions, core.features, core.catalog_sources,
            core.document_types, core.document_type_default_offsets, core.document_type_renewal_steps,
            core.applicability_rules, core.applicability_rule_features
            FROM core_app';
    END IF;
END$$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE core.audit_events;
DROP TABLE core.calendar_exports;
DROP TABLE core.outbox_events;
DROP TABLE core.document_reminder_offsets;
DROP TABLE core.document_periods;
DROP TABLE core.documents;
DROP TABLE core.invites;
DROP TABLE core.memberships;
DROP TABLE core.organization_features;
DROP TABLE core.organizations;
DROP TABLE core.sessions;
DROP TABLE core.accounts;
DROP TABLE core.applicability_rule_features;
DROP TABLE core.applicability_rules;
DROP TABLE core.document_type_renewal_steps;
DROP TABLE core.document_type_default_offsets;
DROP TABLE core.document_types;
DROP TABLE core.catalog_sources;
DROP TABLE core.features;
DROP TABLE core.regions;
DROP TABLE core.business_categories;
