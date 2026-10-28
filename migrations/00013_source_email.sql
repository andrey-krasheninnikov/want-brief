-- +goose Up
ALTER TABLE sources
    ADD COLUMN email TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE sources
    DROP COLUMN IF EXISTS email;
