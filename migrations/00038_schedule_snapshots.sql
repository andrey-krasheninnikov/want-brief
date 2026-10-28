-- +goose Up
CREATE TABLE schedule_snapshots (
    kind TEXT PRIMARY KEY,
    taken_at TIMESTAMPTZ NOT NULL,
    blocks JSONB NOT NULL,
    CONSTRAINT schedule_snapshots_kind_chk CHECK (kind IN ('work', 'followup'))
);

-- +goose Down
DROP TABLE IF EXISTS schedule_snapshots;
