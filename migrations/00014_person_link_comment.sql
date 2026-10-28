-- +goose Up
ALTER TABLE person_projects
    ADD COLUMN comment TEXT NOT NULL DEFAULT '';

ALTER TABLE person_events
    ADD COLUMN comment TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE person_events DROP COLUMN IF EXISTS comment;
ALTER TABLE person_projects DROP COLUMN IF EXISTS comment;
