-- +goose Up
ALTER TABLE items ADD COLUMN pinned_at TIMESTAMPTZ;

ALTER TABLE items DROP CONSTRAINT IF EXISTS items_occupancy_check;
ALTER TABLE items
    ADD CONSTRAINT items_occupancy_check CHECK (occupancy IN ('solo', 'parallel', 'waiting'));

-- +goose Down
UPDATE items SET occupancy = 'parallel' WHERE occupancy = 'waiting';
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_occupancy_check;
ALTER TABLE items
    ADD CONSTRAINT items_occupancy_check CHECK (occupancy IN ('solo', 'parallel'));

ALTER TABLE items DROP COLUMN IF EXISTS pinned_at;
