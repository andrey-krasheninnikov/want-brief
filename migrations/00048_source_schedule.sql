-- +goose Up
ALTER TABLE sources
    ADD COLUMN sync_interval_minutes INTEGER NOT NULL DEFAULT 0
        CHECK (sync_interval_minutes IN (0, 15, 60, 360, 1440)),
    ADD COLUMN last_attempt_at TIMESTAMPTZ,
    ADD COLUMN sync_claim_id UUID,
    ADD COLUMN sync_claim_until TIMESTAMPTZ,
    ADD CONSTRAINT sources_sync_claim_pair CHECK ((sync_claim_id IS NULL) = (sync_claim_until IS NULL));

-- +goose Down
ALTER TABLE sources
    DROP CONSTRAINT sources_sync_claim_pair,
    DROP COLUMN sync_claim_until,
    DROP COLUMN sync_claim_id,
    DROP COLUMN last_attempt_at,
    DROP COLUMN sync_interval_minutes;
