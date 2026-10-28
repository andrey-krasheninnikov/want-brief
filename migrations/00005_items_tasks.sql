-- +goose Up
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_status_check;

UPDATE items SET status = 'backlog' WHERE status = 'open';

ALTER TABLE items
    ADD CONSTRAINT items_status_check CHECK (
        status IN (
            'backlog',
            'in_progress',
            'blocked',
            'review',
            'qa',
            'release_candidate',
            'done'
        )
    );

ALTER TABLE items
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    ADD COLUMN planned_seconds INT NOT NULL DEFAULT 0,
    ADD COLUMN links JSONB NOT NULL DEFAULT '[]';

ALTER TABLE items
    ADD CONSTRAINT items_planned_seconds_chk CHECK (planned_seconds >= 0);

CREATE TABLE item_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX item_notes_item_created_idx ON item_notes (item_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS item_notes_item_created_idx;
DROP TABLE IF EXISTS item_notes;
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_planned_seconds_chk;
ALTER TABLE items
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS planned_seconds,
    DROP COLUMN IF EXISTS links;
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_status_check;
UPDATE items SET status = 'open' WHERE status = 'backlog';
UPDATE items SET status = 'done' WHERE status NOT IN ('open', 'done');
ALTER TABLE items
    ADD CONSTRAINT items_status_check CHECK (status IN ('open', 'done'));
