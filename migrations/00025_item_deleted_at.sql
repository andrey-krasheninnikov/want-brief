-- +goose Up
ALTER TABLE items
    ADD COLUMN deleted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE items
    DROP COLUMN IF EXISTS deleted_at;
