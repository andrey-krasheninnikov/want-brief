-- +goose Up
ALTER TABLE events
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    ADD COLUMN agenda TEXT NOT NULL DEFAULT '',
    ADD COLUMN type TEXT NOT NULL DEFAULT 'sync',
    ADD COLUMN project_id UUID REFERENCES projects (id) ON DELETE SET NULL,
    ADD COLUMN duration_seconds INT NOT NULL DEFAULT 1800,
    ADD COLUMN recurrence TEXT NOT NULL DEFAULT 'once',
    ADD COLUMN links JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN meet_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN involvement INT NOT NULL DEFAULT 5,
    ADD COLUMN active_start_offset INT,
    ADD COLUMN active_end_offset INT,
    ADD COLUMN can_skip BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE events
SET duration_seconds = GREATEST(1, EXTRACT(EPOCH FROM (ends_at - starts_at))::int)
WHERE ends_at IS NOT NULL AND ends_at > starts_at;

UPDATE events
SET ends_at = starts_at + (duration_seconds * INTERVAL '1 second');

ALTER TABLE events
    ADD CONSTRAINT events_type_check CHECK (
        type IN (
            'sync',
            'grooming',
            'lesson',
            'mentorship',
            'planning',
            'daily',
            'retro',
            'one_on_one',
            'global',
            'team_building',
            'external'
        )
    );

ALTER TABLE events
    ADD CONSTRAINT events_recurrence_check CHECK (recurrence IN ('once', 'weekly', 'monthly'));

ALTER TABLE events
    ADD CONSTRAINT events_duration_check CHECK (duration_seconds >= 1);

ALTER TABLE events
    ADD CONSTRAINT events_involvement_check CHECK (involvement BETWEEN 0 AND 10);

ALTER TABLE events
    ADD CONSTRAINT events_active_window_check CHECK (
        (active_start_offset IS NULL AND active_end_offset IS NULL)
        OR (
            active_start_offset IS NOT NULL
            AND active_end_offset IS NOT NULL
            AND active_start_offset >= 0
            AND active_end_offset > active_start_offset
            AND active_end_offset <= duration_seconds
        )
    );

-- +goose Down
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_active_window_check;
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_involvement_check;
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_duration_check;
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_recurrence_check;
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_type_check;
ALTER TABLE events
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS agenda,
    DROP COLUMN IF EXISTS type,
    DROP COLUMN IF EXISTS project_id,
    DROP COLUMN IF EXISTS duration_seconds,
    DROP COLUMN IF EXISTS recurrence,
    DROP COLUMN IF EXISTS links,
    DROP COLUMN IF EXISTS meet_url,
    DROP COLUMN IF EXISTS involvement,
    DROP COLUMN IF EXISTS active_start_offset,
    DROP COLUMN IF EXISTS active_end_offset,
    DROP COLUMN IF EXISTS can_skip,
    DROP COLUMN IF EXISTS updated_at;
