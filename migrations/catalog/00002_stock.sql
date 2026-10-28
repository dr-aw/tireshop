-- +goose Up
ALTER TABLE catalog.products ADD COLUMN stock bigint NOT NULL DEFAULT 0 CHECK (stock BETWEEN 0 AND 9007199254740991);
ALTER TABLE catalog.products ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0);

-- Catalog-owned command API. The caller's transaction also commits the request
-- status; no runtime role receives cross-service table write privileges.
-- +goose StatementBegin
CREATE FUNCTION catalog.dispatch_v1(ids uuid[], quantities bigint[]) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE p record;
BEGIN
    IF cardinality(ids) IS NULL OR cardinality(ids) NOT BETWEEN 1 AND 100
       OR cardinality(ids) <> cardinality(quantities)
       OR EXISTS (SELECT 1 FROM unnest(quantities) q WHERE q IS NULL OR q < 1 OR q > 10000)
       OR EXISTS (SELECT 1 FROM unnest(ids) id WHERE id IS NULL)
       OR (SELECT count(DISTINCT id) FROM unnest(ids) id) <> cardinality(ids) THEN
        RAISE EXCEPTION 'invalid dispatch' USING ERRCODE = '22023';
    END IF;
    FOR p IN SELECT product_id, stock FROM catalog.products
             WHERE product_id = ANY(ids) ORDER BY product_id FOR UPDATE LOOP
        IF p.stock < quantities[array_position(ids, p.product_id)] THEN
            RAISE EXCEPTION 'insufficient stock' USING ERRCODE = 'P0001';
        END IF;
    END LOOP;
    IF (SELECT count(*) FROM catalog.products WHERE product_id = ANY(ids)) <> cardinality(ids) THEN
        RAISE EXCEPTION 'missing product' USING ERRCODE = 'P0001';
    END IF;
    UPDATE catalog.products SET stock = stock - quantities[array_position(ids, product_id)],
        version = version + 1 WHERE product_id = ANY(ids);
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION catalog.dispatch_v1(uuid[], bigint[]) FROM PUBLIC;
-- +goose StatementBegin
DO $$ BEGIN
    EXECUTE format('GRANT EXECUTE ON FUNCTION catalog.dispatch_v1(uuid[], bigint[]) TO %I', current_database() || '_requests');
END $$;
-- +goose StatementEnd
