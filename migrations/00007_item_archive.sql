-- +goose Up
ALTER TABLE items
    ADD COLUMN archived_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE items
    DROP COLUMN IF EXISTS archived_at;
