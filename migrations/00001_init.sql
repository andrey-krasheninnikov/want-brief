-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#4C4CFF',
    target_hours_week NUMERIC(6, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('jira', 'todoist', 'local')),
    name TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    token_sealed TEXT NOT NULL DEFAULT '',
    query_filter TEXT NOT NULL DEFAULT '',
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    external_key TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('open', 'done')),
    kind TEXT NOT NULL CHECK (
        kind IN (
            'task',
            'note',
            'agreement',
            'obligation',
            'initiative',
            'life'
        )
    ),
    project_id UUID REFERENCES projects (id) ON DELETE SET NULL,
    urgent BOOLEAN NOT NULL DEFAULT false,
    important BOOLEAN NOT NULL DEFAULT false,
    stress SMALLINT CHECK (stress IS NULL OR (stress BETWEEN 1 AND 5)),
    due_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX items_source_external_uidx
ON items (source_id, external_key)
WHERE external_key <> '';

CREATE INDEX items_status_idx ON items (status);
CREATE INDEX items_project_idx ON items (project_id);

CREATE TABLE notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID REFERENCES items (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE time_intervals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE INDEX time_intervals_item_idx ON time_intervals (item_id, started_at);
CREATE INDEX time_intervals_open_idx ON time_intervals (ended_at)
WHERE ended_at IS NULL;

CREATE TABLE stress_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID REFERENCES items (id) ON DELETE SET NULL,
    level SMALLINT NOT NULL CHECK (level BETWEEN 1 AND 5),
    logged_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX stress_logs_logged_idx ON stress_logs (logged_at);

-- +goose Down
DROP TABLE IF EXISTS stress_logs;
DROP TABLE IF EXISTS time_intervals;
DROP TABLE IF EXISTS notes;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
