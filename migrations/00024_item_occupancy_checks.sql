-- +goose Up
ALTER TABLE items
    ADD COLUMN occupancy TEXT NOT NULL DEFAULT 'solo',
    ADD COLUMN external_status TEXT NOT NULL DEFAULT '';

ALTER TABLE items
    ADD CONSTRAINT items_occupancy_check CHECK (occupancy IN ('solo', 'parallel'));

CREATE TABLE item_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT false,
    position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT item_checks_body_chk CHECK (btrim(body) <> '')
);

CREATE INDEX item_checks_item_idx ON item_checks (item_id, position, id);

UPDATE items SET status = 'backlog' WHERE status = 'clarification';

ALTER TABLE items DROP CONSTRAINT IF EXISTS items_status_check;
ALTER TABLE items
    ADD CONSTRAINT items_status_check CHECK (
        status IN (
            'backlog',
            'needs_grooming',
            'to_do',
            'in_progress',
            'blocked',
            'review',
            'qa',
            'awaiting_decision',
            'release_candidate',
            'done',
            'cancelled'
        )
    );

-- +goose Down
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_status_check;
ALTER TABLE items
    ADD CONSTRAINT items_status_check CHECK (
        status IN (
            'backlog',
            'clarification',
            'needs_grooming',
            'to_do',
            'in_progress',
            'blocked',
            'review',
            'qa',
            'awaiting_decision',
            'release_candidate',
            'done',
            'cancelled'
        )
    );

DROP INDEX IF EXISTS item_checks_item_idx;
DROP TABLE IF EXISTS item_checks;

ALTER TABLE items DROP CONSTRAINT IF EXISTS items_occupancy_check;
ALTER TABLE items DROP COLUMN IF EXISTS occupancy;
ALTER TABLE items DROP COLUMN IF EXISTS external_status;
