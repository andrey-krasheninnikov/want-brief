-- +goose Up
CREATE TABLE schedule_day_overrides (
    day DATE PRIMARY KEY,
    off BOOLEAN NOT NULL DEFAULT false,
    work_start_min INT,
    work_end_min INT,
    note TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT schedule_day_overrides_window_chk CHECK (
        (work_start_min IS NULL AND work_end_min IS NULL)
        OR (work_start_min IS NOT NULL AND work_end_min IS NOT NULL AND work_start_min < work_end_min)
    )
);

-- +goose Down
DROP TABLE IF EXISTS schedule_day_overrides;
