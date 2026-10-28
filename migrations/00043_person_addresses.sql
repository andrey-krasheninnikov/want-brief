-- +goose Up
CREATE TABLE person_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    label TEXT NOT NULL,
    value TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT person_addresses_kind_chk CHECK (kind IN ('home', 'work', 'other')),
    CONSTRAINT person_addresses_label_chk CHECK (btrim(label) <> ''),
    CONSTRAINT person_addresses_value_chk CHECK (btrim(value) <> '')
);

CREATE INDEX person_addresses_person_idx ON person_addresses (person_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS person_addresses;
