-- +goose Up
ALTER TABLE earn_days
    ADD COLUMN rate NUMERIC(14, 6) NOT NULL DEFAULT 85;
ALTER TABLE earn_days
    ADD CONSTRAINT earn_days_rate_chk CHECK (rate > 0);

-- +goose Down
ALTER TABLE earn_days DROP CONSTRAINT IF EXISTS earn_days_rate_chk;
ALTER TABLE earn_days DROP COLUMN IF EXISTS rate;
