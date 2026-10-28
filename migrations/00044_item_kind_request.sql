-- +goose Up
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_kind_check;
ALTER TABLE items
    ADD CONSTRAINT items_kind_check CHECK (
        kind IN (
            'task',
            'request',
            'note',
            'agreement',
            'obligation',
            'initiative',
            'life'
        )
    );

-- +goose Down
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_kind_check;
UPDATE items SET kind = 'task' WHERE kind = 'request';
ALTER TABLE items
    ADD CONSTRAINT items_kind_check CHECK (
        kind IN (
            'task',
            'note',
            'agreement',
            'obligation',
            'initiative',
            'life'
        )
    );
