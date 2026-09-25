-- Роль приложения не должна изменять историю миграций (ADR-014, находка 7
-- архитектурного анализа). Таблицу core.goose_db_version создаёт goose уже после
-- применения 00001, поэтому права правятся отдельной миграцией.

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'core_app') THEN
        EXECUTE 'REVOKE ALL ON core.goose_db_version FROM core_app';
        BEGIN
            EXECUTE 'REVOKE ALL ON SEQUENCE core.goose_db_version_id_seq FROM core_app';
        EXCEPTION WHEN undefined_table THEN
            NULL;
        END;
        EXECUTE 'GRANT SELECT ON core.goose_db_version TO core_app';
    END IF;
END$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'core_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON core.goose_db_version TO core_app';
    END IF;
END$$;
-- +goose StatementEnd
