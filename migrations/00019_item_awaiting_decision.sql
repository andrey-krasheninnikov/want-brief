-- +goose Up
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_status_check;
ALTER TABLE items
    ADD CONSTRAINT items_status_check CHECK (
        status IN (
            'backlog',
            'clarification',
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
UPDATE items SET status = 'qa' WHERE status = 'awaiting_decision';
ALTER TABLE items
    ADD CONSTRAINT items_status_check CHECK (
        status IN (
            'backlog',
            'clarification',
            'to_do',
            'in_progress',
            'blocked',
            'review',
            'qa',
            'release_candidate',
            'done',
            'cancelled'
        )
    );
