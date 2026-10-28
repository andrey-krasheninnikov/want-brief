-- +goose Up
ALTER TABLE sources
    ADD COLUMN project_id UUID REFERENCES projects (id) ON DELETE SET NULL,
    ADD COLUMN connected BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN last_error TEXT NOT NULL DEFAULT '';

CREATE INDEX sources_project_idx ON sources (project_id);

-- +goose Down
DROP INDEX IF EXISTS sources_project_idx;

ALTER TABLE sources
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS connected,
    DROP COLUMN IF EXISTS project_id;
