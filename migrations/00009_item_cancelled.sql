-- +goose Up
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_status_check;
ALTER TABLE items
    ADD CONSTRAINT items_status_check CHECK (
        status IN (
            'backlog',
            'in_progress',
            'blocked',
            'review',
            'qa',
            'release_candidate',
            'done',
            'cancelled'
        )
    );

-- +goose Down
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_status_check;
UPDATE items SET status = 'done' WHERE status = 'cancelled';
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
