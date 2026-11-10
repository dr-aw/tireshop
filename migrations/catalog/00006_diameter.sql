-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
ALTER TABLE catalog.tyre_specs RENAME COLUMN radius TO diameter;
ALTER TABLE catalog.wheel_specs RENAME COLUMN radius TO diameter;
-- Existing generated expressions and indexes follow column renames automatically.
-- Keep radius in the view as a compatibility alias.
CREATE OR REPLACE VIEW catalog.product_details AS
 SELECT p.*,coalesce(t.size,w.marking,'') AS size,coalesce(t.season,'') AS season,
 t.tread_percent,coalesce(t.load_index,'') AS load_index,coalesce(t.speed_index,'') AS speed_index,
 coalesce(t.diameter,w.diameter) AS diameter,w.width_inches AS width,w.offset_mm AS wheel_offset,
 coalesce(w.pcd,'') AS pcd,coalesce(w.wheel_type,'') AS wheel_type,w.center_bore_mm AS center_bore,
 t.width_value AS tyre_width,coalesce(t.width_unit,'') AS tyre_width_unit,
 t.aspect_ratio,t.rim_diameter_inches AS rim_diameter,
 coalesce(t.diameter,w.diameter) AS radius,t.size_key
 FROM catalog.products p LEFT JOIN catalog.tyre_specs t USING(product_id)
 LEFT JOIN catalog.wheel_specs w USING(product_id);
