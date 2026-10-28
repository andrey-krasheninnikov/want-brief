-- +goose Up
DROP TABLE IF EXISTS company_items;

-- +goose Down
CREATE TABLE company_items (
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    PRIMARY KEY (company_id, item_id)
);

CREATE INDEX company_items_item_idx ON company_items (item_id);
