-- +goose Up
ALTER TABLE projects
    ADD COLUMN monthly_income_usd NUMERIC(12, 2) NOT NULL DEFAULT 0,
    ADD COLUMN monthly_income_rub NUMERIC(12, 2) NOT NULL DEFAULT 0;

UPDATE projects
SET monthly_income_rub = monthly_income
WHERE monthly_income > 0;

ALTER TABLE projects
    ADD CONSTRAINT projects_income_usd_chk CHECK (monthly_income_usd >= 0),
    ADD CONSTRAINT projects_income_rub_chk CHECK (monthly_income_rub >= 0);

CREATE TABLE project_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX project_notes_project_created_idx ON project_notes (project_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS project_notes_project_created_idx;
DROP TABLE IF EXISTS project_notes;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_income_usd_chk;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_income_rub_chk;
ALTER TABLE projects
    DROP COLUMN IF EXISTS monthly_income_usd,
    DROP COLUMN IF EXISTS monthly_income_rub;
