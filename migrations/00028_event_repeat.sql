-- +goose Up
ALTER TABLE events
    ADD COLUMN repeat_until DATE,
    ADD COLUMN weekdays SMALLINT[] NOT NULL DEFAULT '{}';

UPDATE events
SET weekdays = ARRAY[
    ((EXTRACT(DOW FROM (starts_at AT TIME ZONE 'Europe/Moscow'))::int + 6) % 7),
    ((EXTRACT(DOW FROM (starts_at AT TIME ZONE 'Europe/Moscow'))::int + 6) % 7) + 7
]::smallint[]
WHERE recurrence = 'weekly';

ALTER TABLE events
    ADD CONSTRAINT events_weekly_weekdays_chk CHECK (
        (recurrence <> 'weekly' AND cardinality(weekdays) = 0)
        OR (recurrence = 'weekly' AND cardinality(weekdays) >= 1)
    );

CREATE TABLE event_overrides (
    series_id UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    original_on DATE NOT NULL,
    starts_at TIMESTAMPTZ,
    duration_seconds INT,
    skipped BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (series_id, original_on),
    CONSTRAINT event_overrides_duration_chk CHECK (duration_seconds IS NULL OR duration_seconds >= 1)
);

CREATE TABLE event_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    series_id UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    original_on DATE NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT event_notes_body_chk CHECK (btrim(body) <> '')
);

CREATE INDEX event_notes_series_created_idx ON event_notes (series_id, created_at DESC);
CREATE INDEX event_notes_series_on_idx ON event_notes (series_id, original_on);

-- +goose Down
DROP INDEX IF EXISTS event_notes_series_on_idx;
DROP INDEX IF EXISTS event_notes_series_created_idx;
DROP TABLE IF EXISTS event_notes;
DROP TABLE IF EXISTS event_overrides;
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_weekly_weekdays_chk;
ALTER TABLE events
    DROP COLUMN IF EXISTS weekdays,
    DROP COLUMN IF EXISTS repeat_until;
