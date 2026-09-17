-- +goose Up
CREATE TABLE person_absences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT person_absences_range_chk CHECK (ends_on >= starts_on)
);

CREATE INDEX person_absences_person_idx ON person_absences (person_id, starts_on);
CREATE INDEX person_absences_range_idx ON person_absences (starts_on, ends_on);

-- +goose Down
DROP TABLE IF EXISTS person_absences;
