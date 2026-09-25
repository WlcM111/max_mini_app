-- Роль приложения не должна изменять историю миграций (ADR-014, находка 7
-- архитектурного анализа). Таблицу bot.goose_db_version создаёт goose уже после
-- применения 00001, поэтому права правятся отдельной миграцией.
-- Роли может не быть в автономной тестовой среде.

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bot_app') THEN
        EXECUTE 'REVOKE ALL ON bot.goose_db_version FROM bot_app';
        BEGIN
            EXECUTE 'REVOKE ALL ON SEQUENCE bot.goose_db_version_id_seq FROM bot_app';
        EXCEPTION WHEN undefined_table THEN
            NULL;
        END;
        EXECUTE 'GRANT SELECT ON bot.goose_db_version TO bot_app';
    END IF;
END$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bot_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON bot.goose_db_version TO bot_app';
    END IF;
END$$;
-- +goose StatementEnd
