-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
-- Versioned normalization leaves the original size and parser unchanged.
-- +goose StatementBegin
CREATE FUNCTION catalog.tyre_size_key_v1(size text) RETURNS text
LANGUAGE plpgsql IMMUTABLE STRICT PARALLEL SAFE SET search_path=pg_catalog AS $$
DECLARE d jsonb; width numeric; label text;
BEGIN
 d := catalog.tyre_dimensions(size);
 IF d->>'rim' IS NULL THEN RETURN NULL; END IF;
 width := (d->>'width')::numeric;
 IF d->>'unit' = 'in' THEN
  -- Preserve meaningful extra decimals; customary inch widths use two places.
  label := CASE WHEN scale(trim_scale(width)) <= 2 THEN width::numeric(8,2)::text
                ELSE trim_scale(width)::text END;
 ELSE label := trim_scale(width)::text;
 END IF;
 IF d->>'profile' IS NOT NULL THEN label := label || '/' || trim_scale((d->>'profile')::numeric)::text; END IF;
 RETURN label || 'R' || trim_scale((d->>'rim')::numeric)::text;
END $$;
-- +goose StatementEnd
ALTER TABLE catalog.tyre_specs
 ADD COLUMN radius numeric GENERATED ALWAYS AS ((catalog.tyre_dimensions(size)->>'rim')::numeric) STORED,
 ADD COLUMN size_key text GENERATED ALWAYS AS (catalog.tyre_size_key_v1(size)) STORED;
ALTER TABLE catalog.wheel_specs
 ADD COLUMN radius numeric GENERATED ALWAYS AS (diameter_inches) STORED;
CREATE INDEX tyre_specs_selection_idx ON catalog.tyre_specs(radius,size_key,product_id);
CREATE INDEX wheel_specs_selection_idx ON catalog.wheel_specs(radius,width_inches,product_id);
-- Append fields: old readers and jsonb_populate_record writers remain compatible.
CREATE OR REPLACE VIEW catalog.product_details AS
 SELECT p.*,coalesce(t.size,w.marking,'') AS size,coalesce(t.season,'') AS season,
 t.tread_percent,coalesce(t.load_index,'') AS load_index,coalesce(t.speed_index,'') AS speed_index,
 w.diameter_inches AS diameter,w.width_inches AS width,w.offset_mm AS wheel_offset,
 coalesce(w.pcd,'') AS pcd,coalesce(w.wheel_type,'') AS wheel_type,w.center_bore_mm AS center_bore,
 t.width_value AS tyre_width,coalesce(t.width_unit,'') AS tyre_width_unit,
 t.aspect_ratio,t.rim_diameter_inches AS rim_diameter,
 coalesce(t.radius,w.radius) AS radius,t.size_key
 FROM catalog.products p LEFT JOIN catalog.tyre_specs t USING(product_id)
 LEFT JOIN catalog.wheel_specs w USING(product_id);
