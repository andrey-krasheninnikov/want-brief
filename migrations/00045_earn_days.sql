-- +goose Up
CREATE TABLE earn_days (
    day DATE PRIMARY KEY,
    usd NUMERIC(14, 6) NOT NULL DEFAULT 0,
    rub NUMERIC(14, 6) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT earn_days_usd_chk CHECK (usd >= 0),
    CONSTRAINT earn_days_rub_chk CHECK (rub >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS earn_days;
