-- +goose Up
CREATE TABLE schedule_settings (
    id SMALLINT PRIMARY KEY,
    data JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT schedule_settings_singleton_chk CHECK (id = 1)
);

-- +goose Down
DROP TABLE IF EXISTS schedule_settings;
