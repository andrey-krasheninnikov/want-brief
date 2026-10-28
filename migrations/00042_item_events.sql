-- +goose Up
CREATE TABLE item_events (
    id UUID PRIMARY KEY,
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    field TEXT NOT NULL DEFAULT '',
    from_value TEXT NOT NULL DEFAULT '',
    to_value TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    stress INT,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX item_events_item_created_idx ON item_events (item_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS item_events_item_created_idx;
DROP TABLE IF EXISTS item_events;
