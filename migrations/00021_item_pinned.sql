-- +goose Up
ALTER TABLE items
    ADD COLUMN pinned BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE items
    DROP COLUMN IF EXISTS pinned;
