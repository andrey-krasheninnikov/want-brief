-- +goose Up
ALTER TABLE projects
    ADD COLUMN archived_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE projects
    DROP COLUMN IF EXISTS archived_at;
