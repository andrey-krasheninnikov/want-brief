-- +goose Up
UPDATE people
SET born_on = (CURRENT_DATE - (age_years || ' years')::interval)::date
WHERE born_on IS NULL AND age_years IS NOT NULL;

ALTER TABLE people DROP CONSTRAINT IF EXISTS people_age_xor_chk;
ALTER TABLE people DROP CONSTRAINT IF EXISTS people_age_years_chk;
ALTER TABLE people DROP COLUMN age_years;

CREATE TABLE person_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    label TEXT NOT NULL,
    value TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT person_contacts_kind_chk CHECK (kind IN ('phone', 'telegram', 'url')),
    CONSTRAINT person_contacts_label_chk CHECK (btrim(label) <> ''),
    CONSTRAINT person_contacts_value_chk CHECK (btrim(value) <> '')
);

CREATE INDEX person_contacts_person_idx ON person_contacts (person_id, created_at);

CREATE TABLE person_sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    url TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT person_sites_kind_chk CHECK (kind IN (
        'personal_site', 'company_site', 'github', 'linkedin', 'youtube', 'telegram_channel'
    )),
    CONSTRAINT person_sites_url_chk CHECK (btrim(url) <> '')
);

CREATE INDEX person_sites_person_idx ON person_sites (person_id, created_at);

CREATE TABLE person_bonds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_a_id UUID REFERENCES people (id) ON DELETE CASCADE,
    person_b_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    started_on DATE NOT NULL,
    changed_on DATE NOT NULL,
    ended_on DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT person_bonds_kind_chk CHECK (kind IN (
        'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary'
    )),
    CONSTRAINT person_bonds_pair_chk CHECK (person_a_id IS DISTINCT FROM person_b_id),
    CONSTRAINT person_bonds_dates_chk CHECK (
        changed_on >= started_on AND (ended_on IS NULL OR ended_on >= started_on)
    )
);

CREATE UNIQUE INDEX person_bonds_open_pair_idx ON person_bonds (
    COALESCE(person_a_id, '00000000-0000-0000-0000-000000000000'::uuid),
    person_b_id
) WHERE ended_on IS NULL;

CREATE INDEX person_bonds_person_b_idx ON person_bonds (person_b_id);
CREATE INDEX person_bonds_person_a_idx ON person_bonds (person_a_id);

CREATE TABLE person_bond_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bond_id UUID NOT NULL REFERENCES person_bonds (id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    kind TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    started_on DATE NOT NULL,
    changed_on DATE NOT NULL,
    ended_on DATE,
    at TIMESTAMPTZ NOT NULL,
    CONSTRAINT person_bond_events_action_chk CHECK (action IN ('open', 'change', 'end')),
    CONSTRAINT person_bond_events_kind_chk CHECK (kind IN (
        'acquaintance', 'comrade', 'friend', 'relative', 'spouse', 'adversary'
    ))
);

CREATE INDEX person_bond_events_bond_idx ON person_bond_events (bond_id, at);

-- +goose Down
DROP TABLE IF EXISTS person_bond_events;
DROP TABLE IF EXISTS person_bonds;
DROP TABLE IF EXISTS person_sites;
DROP TABLE IF EXISTS person_contacts;
ALTER TABLE people ADD COLUMN age_years SMALLINT;
