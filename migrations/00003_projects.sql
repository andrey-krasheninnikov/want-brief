-- +goose Up
ALTER TABLE projects
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    ADD COLUMN monthly_income NUMERIC(12, 2) NOT NULL DEFAULT 0,
    ADD COLUMN target_hours_day NUMERIC(6, 2) NOT NULL DEFAULT 0,
    ADD COLUMN notes TEXT NOT NULL DEFAULT '',
    ADD COLUMN links JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE projects
SET target_hours_day = round(target_hours_week / 7, 2)
WHERE target_hours_week > 0;

ALTER TABLE projects
    ADD CONSTRAINT projects_income_chk CHECK (monthly_income >= 0),
    ADD CONSTRAINT projects_day_chk CHECK (target_hours_day >= 0);

-- +goose Down
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_income_chk;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_day_chk;
ALTER TABLE projects
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS monthly_income,
    DROP COLUMN IF EXISTS target_hours_day,
    DROP COLUMN IF EXISTS notes,
    DROP COLUMN IF EXISTS links;
