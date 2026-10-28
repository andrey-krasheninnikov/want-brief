-- +goose Up
CREATE TABLE person_professions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    started_on DATE NOT NULL,
    ended_on DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT person_professions_title_chk CHECK (btrim(title) <> ''),
    CONSTRAINT person_professions_dates_chk CHECK (ended_on IS NULL OR ended_on >= started_on)
);

CREATE INDEX person_professions_person_idx ON person_professions (person_id, started_on, id);

CREATE TABLE person_profession_salaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profession_id UUID NOT NULL REFERENCES person_professions (id) ON DELETE CASCADE,
    started_on DATE NOT NULL,
    ended_on DATE,
    monthly_salary_usd NUMERIC(12, 2) NOT NULL DEFAULT 0,
    monthly_salary_rub NUMERIC(12, 2) NOT NULL DEFAULT 0,
    CONSTRAINT person_profession_salaries_dates_chk CHECK (ended_on IS NULL OR ended_on >= started_on),
    CONSTRAINT person_profession_salaries_usd_chk CHECK (monthly_salary_usd >= 0),
    CONSTRAINT person_profession_salaries_rub_chk CHECK (monthly_salary_rub >= 0)
);

CREATE INDEX person_profession_salaries_prof_idx ON person_profession_salaries (profession_id, started_on, id);
CREATE UNIQUE INDEX person_profession_salaries_open_idx ON person_profession_salaries (profession_id) WHERE ended_on IS NULL;

INSERT INTO person_professions (person_id, title, comment, started_on, ended_on, created_at, updated_at)
SELECT
    id,
    CASE WHEN btrim(profession) = '' THEN 'Profession' ELSE profession END,
    '',
    created_at::date,
    NULL,
    created_at,
    updated_at
FROM people
WHERE btrim(profession) <> '' OR monthly_salary_usd > 0 OR monthly_salary_rub > 0;

INSERT INTO person_profession_salaries (profession_id, started_on, ended_on, monthly_salary_usd, monthly_salary_rub)
SELECT pp.id, pp.started_on, NULL, p.monthly_salary_usd, p.monthly_salary_rub
FROM person_professions pp
JOIN people p ON p.id = pp.person_id;

ALTER TABLE people DROP COLUMN profession;
ALTER TABLE people DROP COLUMN monthly_salary_usd;
ALTER TABLE people DROP COLUMN monthly_salary_rub;

ALTER TABLE person_bonds DROP CONSTRAINT person_bonds_kind_chk;
ALTER TABLE person_bonds ADD CONSTRAINT person_bonds_kind_chk CHECK (kind IN (
    'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary', 'other'
));

ALTER TABLE person_bond_events DROP CONSTRAINT person_bond_events_kind_chk;
ALTER TABLE person_bond_events ADD CONSTRAINT person_bond_events_kind_chk CHECK (kind IN (
    'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary', 'other'
));

ALTER TABLE person_sites DROP CONSTRAINT person_sites_kind_chk;
ALTER TABLE person_sites ADD CONSTRAINT person_sites_kind_chk CHECK (kind IN (
    'personal_site', 'company_site', 'github', 'linkedin', 'youtube', 'telegram_channel', 'other'
));

-- +goose Down
ALTER TABLE person_sites DROP CONSTRAINT person_sites_kind_chk;
ALTER TABLE person_sites ADD CONSTRAINT person_sites_kind_chk CHECK (kind IN (
    'personal_site', 'company_site', 'github', 'linkedin', 'youtube', 'telegram_channel'
));

ALTER TABLE person_bond_events DROP CONSTRAINT person_bond_events_kind_chk;
ALTER TABLE person_bond_events ADD CONSTRAINT person_bond_events_kind_chk CHECK (kind IN (
    'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary'
));

ALTER TABLE person_bonds DROP CONSTRAINT person_bonds_kind_chk;
ALTER TABLE person_bonds ADD CONSTRAINT person_bonds_kind_chk CHECK (kind IN (
    'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary'
));

ALTER TABLE people
    ADD COLUMN profession TEXT NOT NULL DEFAULT '',
    ADD COLUMN monthly_salary_usd NUMERIC(12, 2) NOT NULL DEFAULT 0,
    ADD COLUMN monthly_salary_rub NUMERIC(12, 2) NOT NULL DEFAULT 0;

UPDATE people p SET
    profession = COALESCE((
        SELECT pp.title
        FROM person_professions pp
        WHERE pp.person_id = p.id
        ORDER BY pp.ended_on NULLS FIRST, pp.started_on DESC, pp.id
        LIMIT 1
    ), ''),
    monthly_salary_usd = COALESCE((
        SELECT s.monthly_salary_usd
        FROM person_professions pp
        JOIN person_profession_salaries s ON s.profession_id = pp.id
        WHERE pp.person_id = p.id AND s.ended_on IS NULL
        ORDER BY pp.started_on DESC, pp.id
        LIMIT 1
    ), 0),
    monthly_salary_rub = COALESCE((
        SELECT s.monthly_salary_rub
        FROM person_professions pp
        JOIN person_profession_salaries s ON s.profession_id = pp.id
        WHERE pp.person_id = p.id AND s.ended_on IS NULL
        ORDER BY pp.started_on DESC, pp.id
        LIMIT 1
    ), 0);

DROP TABLE IF EXISTS person_profession_salaries;
DROP TABLE IF EXISTS person_professions;
