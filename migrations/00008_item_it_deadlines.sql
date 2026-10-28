-- +goose Up
ALTER TABLE items
    ADD COLUMN review_due_at TIMESTAMPTZ,
    ADD COLUMN test_due_at TIMESTAMPTZ;

ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_kind_check;
UPDATE sources SET kind = 'manual', name = 'Manual' WHERE kind = 'local';
ALTER TABLE sources ADD CONSTRAINT sources_kind_check CHECK (kind IN ('jira', 'todoist', 'manual'));

-- +goose Down
ALTER TABLE items
    DROP COLUMN IF EXISTS review_due_at,
    DROP COLUMN IF EXISTS test_due_at;

ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_kind_check;
UPDATE sources SET kind = 'local', name = 'Local' WHERE kind = 'manual';
ALTER TABLE sources ADD CONSTRAINT sources_kind_check CHECK (kind IN ('jira', 'todoist', 'local'));
