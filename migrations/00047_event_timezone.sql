-- +goose Up
ALTER TABLE events ADD COLUMN timezone TEXT NOT NULL DEFAULT 'Europe/Moscow';

-- +goose Down
ALTER TABLE events DROP COLUMN timezone;
