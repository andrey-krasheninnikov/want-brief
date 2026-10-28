-- +goose Up
ALTER TABLE sources
    ADD COLUMN insecure_tls BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE sources
    DROP COLUMN IF EXISTS insecure_tls;
