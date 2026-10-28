-- +goose Up
CREATE TABLE people (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    born_on DATE,
    age_years SMALLINT,
    profession TEXT NOT NULL DEFAULT '',
    monthly_salary_usd NUMERIC(12, 2) NOT NULL DEFAULT 0,
    monthly_salary_rub NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT people_name_chk CHECK (btrim(name) <> ''),
    CONSTRAINT people_age_xor_chk CHECK (NOT (born_on IS NOT NULL AND age_years IS NOT NULL)),
    CONSTRAINT people_age_years_chk CHECK (age_years IS NULL OR (age_years >= 0 AND age_years <= 150)),
    CONSTRAINT people_salary_usd_chk CHECK (monthly_salary_usd >= 0),
    CONSTRAINT people_salary_rub_chk CHECK (monthly_salary_rub >= 0)
);

CREATE TABLE person_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX person_notes_person_created_idx ON person_notes (person_id, created_at DESC);

CREATE TABLE person_projects (
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    PRIMARY KEY (person_id, project_id)
);

CREATE INDEX person_projects_project_idx ON person_projects (project_id);

CREATE TABLE person_events (
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    PRIMARY KEY (person_id, event_id)
);

CREATE INDEX person_events_event_idx ON person_events (event_id);

CREATE TABLE person_items (
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    PRIMARY KEY (person_id, item_id)
);

CREATE INDEX person_items_item_idx ON person_items (item_id);

-- +goose Down
DROP TABLE IF EXISTS person_items;
DROP TABLE IF EXISTS person_events;
DROP TABLE IF EXISTS person_projects;
DROP TABLE IF EXISTS person_notes;
DROP TABLE IF EXISTS people;
