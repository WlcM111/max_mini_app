-- bot schema v1. Выполняется ролью bot_migrator (владелец схемы bot).

-- +goose Up
-- Журнал дедупликации входящих событий webhook (хранится 7 суток, без текста сообщений)
CREATE TABLE bot.inbound_updates (
    dedupe_key  bytea PRIMARY KEY CHECK (octet_length(dedupe_key) = 32),
    update_type text NOT NULL CHECK (char_length(update_type) BETWEEN 1 AND 64),
    event_time  timestamptz NOT NULL,
    max_user_id bigint,
    outcome     text NOT NULL CHECK (outcome IN ('applied', 'ignored')),
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inbound_updates_received_idx ON bot.inbound_updates (received_at);

-- Состояние диалога пользователя с ботом
CREATE TABLE bot.recipients (
    max_user_id      bigint PRIMARY KEY,
    state            text NOT NULL CHECK (state IN ('unknown', 'active', 'muted', 'stopped', 'unreachable')),
    state_changed_at timestamptz NOT NULL,
    last_event_time  timestamptz,
    last_delivery_at timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- Очередь исходящих сообщений (transactional outbox бота)
CREATE TABLE bot.outbound_messages (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id             uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    idempotency_key       text NOT NULL UNIQUE CHECK (idempotency_key ~ '^[a-z0-9:_-]{8,200}$'),
    request_hash          bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    kind                  text NOT NULL CHECK (kind IN ('reminder', 'member_joined', 'welcome', 'help')),
    recipient_max_user_id bigint NOT NULL,
    text                  text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 4000),
    silent                boolean NOT NULL DEFAULT false,
    status                text NOT NULL CHECK (status IN ('queued', 'sending', 'sent', 'retry_wait', 'failed', 'expired')),
    attempts              smallint NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at       timestamptz NOT NULL DEFAULT now(),
    not_after             timestamptz NOT NULL,
    locked_until          timestamptz,
    max_message_id        text CHECK (char_length(max_message_id) <= 128),
    last_error_code       text CHECK (char_length(last_error_code) <= 64),
    created_at            timestamptz NOT NULL DEFAULT now(),
    sent_at               timestamptz,
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'sending') = (locked_until IS NOT NULL)),
    CHECK ((status = 'sent') = (sent_at IS NOT NULL))
);
CREATE INDEX outbound_ready_idx ON bot.outbound_messages (next_attempt_at) WHERE status IN ('queued', 'retry_wait');
CREATE INDEX outbound_lease_idx ON bot.outbound_messages (locked_until) WHERE status = 'sending';
CREATE INDEX outbound_recipient_idx ON bot.outbound_messages (recipient_max_user_id, created_at DESC);
CREATE INDEX outbound_final_idx ON bot.outbound_messages (updated_at) WHERE status IN ('sent', 'failed', 'expired');

CREATE TABLE bot.outbound_buttons (
    message_id       bigint NOT NULL REFERENCES bot.outbound_messages (id) ON DELETE CASCADE,
    position         smallint NOT NULL CHECK (position BETWEEN 1 AND 3),
    text             text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 64),
    action           text NOT NULL CHECK (action IN ('open_app', 'url')),
    -- Эквивалент '^[A-Za-z0-9_-]{0,512}$' (F-05): PostgreSQL ограничивает
    -- счётчик повторов регулярного выражения значением 255, поэтому длина
    -- проверяется отдельно (исправление дефекта D-3 этапа 2).
    open_app_payload text CHECK (open_app_payload ~ '^[A-Za-z0-9_-]*$'
                                 AND char_length(open_app_payload) <= 512),
    url              text CHECK (url ~ '^https://' AND char_length(url) <= 2048),
    PRIMARY KEY (message_id, position),
    CHECK ((action = 'open_app' AND open_app_payload IS NOT NULL AND url IS NULL)
        OR (action = 'url' AND url IS NOT NULL AND open_app_payload IS NULL))
);

-- +goose Down
DROP TABLE bot.outbound_buttons;
DROP TABLE bot.outbound_messages;
DROP TABLE bot.recipients;
DROP TABLE bot.inbound_updates;
