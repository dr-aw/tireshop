-- +goose Up
CREATE TABLE catalog.products (
    product_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 300),
    unit_price_rub bigint NOT NULL CHECK (unit_price_rub BETWEEN 0 AND 9007199254740991),
    is_available boolean NOT NULL DEFAULT true
);

-- This is the only catalog read contract exposed to requests.
CREATE VIEW catalog.request_products_v1 AS
SELECT product_id, name, unit_price_rub, is_available FROM catalog.products;

-- +goose StatementBegin
DO $$
BEGIN
    EXECUTE format('GRANT USAGE ON SCHEMA catalog TO %I, %I',
        current_database() || '_catalog', current_database() || '_requests');
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON catalog.products TO %I',
        current_database() || '_catalog');
    EXECUTE format('GRANT SELECT ON catalog.request_products_v1 TO %I',
        current_database() || '_requests');
END $$;
-- +goose StatementEnd
