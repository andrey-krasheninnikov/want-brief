-- +goose Up
CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    links JSONB NOT NULL DEFAULT '[]'::jsonb,
    started_on DATE,
    ended_on DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT companies_name_chk CHECK (btrim(name) <> ''),
    CONSTRAINT companies_dates_chk CHECK (ended_on IS NULL OR started_on IS NOT NULL),
    CONSTRAINT companies_span_chk CHECK (ended_on IS NULL OR started_on IS NULL OR ended_on >= started_on)
);

CREATE TABLE company_projects (
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    comment TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (company_id, project_id)
);

CREATE INDEX company_projects_project_idx ON company_projects (project_id);

CREATE TABLE company_events (
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    comment TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (company_id, event_id)
);

CREATE INDEX company_events_event_idx ON company_events (event_id);

CREATE TABLE company_items (
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    PRIMARY KEY (company_id, item_id)
);

CREATE INDEX company_items_item_idx ON company_items (item_id);

CREATE TABLE company_people (
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    comment TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (company_id, person_id)
);

CREATE INDEX company_people_person_idx ON company_people (person_id);

CREATE TABLE company_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT company_notes_body_chk CHECK (btrim(body) <> '')
);

CREATE INDEX company_notes_company_created_idx ON company_notes (company_id, created_at DESC);

CREATE TABLE company_titles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    started_on DATE NOT NULL,
    ended_on DATE,
    CONSTRAINT company_titles_title_chk CHECK (btrim(title) <> ''),
    CONSTRAINT company_titles_dates_chk CHECK (ended_on IS NULL OR ended_on >= started_on)
);

CREATE INDEX company_titles_company_idx ON company_titles (company_id, started_on, id);
CREATE UNIQUE INDEX company_titles_open_idx ON company_titles (company_id) WHERE ended_on IS NULL;

CREATE TABLE company_salaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    currency TEXT NOT NULL,
    amount NUMERIC(12, 2) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    started_on DATE NOT NULL,
    ended_on DATE,
    CONSTRAINT company_salaries_currency_chk CHECK (currency IN ('usd', 'rub')),
    CONSTRAINT company_salaries_amount_chk CHECK (amount >= 0),
    CONSTRAINT company_salaries_dates_chk CHECK (ended_on IS NULL OR ended_on >= started_on)
);

CREATE INDEX company_salaries_company_idx ON company_salaries (company_id, started_on, id);
CREATE UNIQUE INDEX company_salaries_open_idx ON company_salaries (company_id) WHERE ended_on IS NULL;

CREATE TABLE company_managers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    started_on DATE NOT NULL,
    ended_on DATE,
    CONSTRAINT company_managers_dates_chk CHECK (ended_on IS NULL OR ended_on >= started_on)
);

CREATE INDEX company_managers_company_idx ON company_managers (company_id, started_on, id);
CREATE UNIQUE INDEX company_managers_open_idx ON company_managers (company_id) WHERE ended_on IS NULL;

CREATE TABLE company_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    started_on DATE NOT NULL,
    ended_on DATE,
    CONSTRAINT company_reports_dates_chk CHECK (ended_on IS NULL OR ended_on >= started_on)
);

CREATE INDEX company_reports_company_idx ON company_reports (company_id, started_on, id);
CREATE UNIQUE INDEX company_reports_open_idx ON company_reports (company_id, person_id) WHERE ended_on IS NULL;

CREATE TABLE company_contracts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    person_id UUID REFERENCES people (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    started_on DATE NOT NULL,
    ended_on DATE,
    CONSTRAINT company_contracts_kind_chk CHECK (kind IN ('informal', 'gph', 'ip', 'labor', 'contract')),
    CONSTRAINT company_contracts_dates_chk CHECK (ended_on IS NULL OR ended_on >= started_on)
);

CREATE INDEX company_contracts_company_idx ON company_contracts (company_id, started_on, id);
CREATE UNIQUE INDEX company_contracts_me_open_idx ON company_contracts (company_id) WHERE person_id IS NULL AND ended_on IS NULL;
CREATE UNIQUE INDEX company_contracts_person_open_idx ON company_contracts (company_id, person_id) WHERE person_id IS NOT NULL AND ended_on IS NULL;

-- +goose Down
DROP TABLE IF EXISTS company_contracts;
DROP TABLE IF EXISTS company_reports;
DROP TABLE IF EXISTS company_managers;
DROP TABLE IF EXISTS company_salaries;
DROP TABLE IF EXISTS company_titles;
DROP TABLE IF EXISTS company_notes;
DROP TABLE IF EXISTS company_people;
DROP TABLE IF EXISTS company_items;
DROP TABLE IF EXISTS company_events;
DROP TABLE IF EXISTS company_projects;
DROP TABLE IF EXISTS companies;
