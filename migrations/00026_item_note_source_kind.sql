-- +goose Up
ALTER TABLE item_notes
    ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'manual';

ALTER TABLE item_notes
    ADD CONSTRAINT item_notes_source_kind_chk CHECK (source_kind IN ('manual', 'jira', 'todoist'));

-- +goose Down
ALTER TABLE item_notes DROP CONSTRAINT IF EXISTS item_notes_source_kind_chk;
ALTER TABLE item_notes DROP COLUMN IF EXISTS source_kind;
