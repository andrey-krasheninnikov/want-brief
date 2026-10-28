-- +goose Up
ALTER TABLE stress_logs
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'stress';

ALTER TABLE stress_logs
    ADD CONSTRAINT stress_logs_kind_chk
    CHECK (kind IN ('stress', 'focus', 'energy', 'interest'));

CREATE INDEX stress_logs_kind_logged_idx ON stress_logs (kind, logged_at DESC);

CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('event', 'call')),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at IS NULL OR ends_at >= starts_at)
);

CREATE INDEX events_starts_idx ON events (starts_at);

-- +goose Down
DROP TABLE IF EXISTS events;
DROP INDEX IF EXISTS stress_logs_kind_logged_idx;
ALTER TABLE stress_logs DROP CONSTRAINT IF EXISTS stress_logs_kind_chk;
ALTER TABLE stress_logs DROP COLUMN IF EXISTS kind;
