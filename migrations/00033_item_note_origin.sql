-- +goose Up
ALTER TABLE item_notes
    ADD COLUMN external_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN author_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN url TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX item_notes_external_idx
    ON item_notes (item_id, external_id)
    WHERE external_id <> '';

-- +goose Down
DROP INDEX IF EXISTS item_notes_external_idx;
ALTER TABLE item_notes
    DROP COLUMN IF EXISTS url,
    DROP COLUMN IF EXISTS author_name,
    DROP COLUMN IF EXISTS external_id;
