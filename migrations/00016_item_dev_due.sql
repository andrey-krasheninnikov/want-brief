-- +goose Up
ALTER TABLE items
    ADD COLUMN dev_due_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE items
    DROP COLUMN IF EXISTS dev_due_at;
