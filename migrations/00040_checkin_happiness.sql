-- +goose Up
ALTER TABLE stress_logs DROP CONSTRAINT IF EXISTS stress_logs_kind_chk;
ALTER TABLE stress_logs
    ADD CONSTRAINT stress_logs_kind_chk
    CHECK (kind IN ('stress', 'focus', 'energy', 'interest', 'happiness'));

-- +goose Down
DELETE FROM stress_logs WHERE kind = 'happiness';
ALTER TABLE stress_logs DROP CONSTRAINT IF EXISTS stress_logs_kind_chk;
ALTER TABLE stress_logs
    ADD CONSTRAINT stress_logs_kind_chk
    CHECK (kind IN ('stress', 'focus', 'energy', 'interest'));
