-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
-- Preserve every removed value for migration audit/recovery, including any
-- historically misplaced attributes. Runtime cannot modify this snapshot.
CREATE TABLE catalog.specs_migration_archive AS
 SELECT product_id,kind,jsonb_build_object('size',size,'season',season,
 'tread_percent',tread_percent,'load_index',load_index,'speed_index',speed_index,
 'diameter',diameter,'width',width,'wheel_offset',wheel_offset,'pcd',pcd,
 'wheel_type',wheel_type,'center_bore',center_bore) AS attributes
 FROM catalog.products;
ALTER TABLE catalog.specs_migration_archive ADD PRIMARY KEY(product_id);

-- The source size remains authoritative. Only complete, recognized formats
-- produce dimensions; missing profiles are never invented.
-- +goose StatementBegin
CREATE FUNCTION catalog.tyre_dimensions(size text) RETURNS jsonb
LANGUAGE plpgsql IMMUTABLE STRICT PARALLEL SAFE SET search_path=pg_catalog AS $$
DECLARE s text; m text[];
BEGIN
 s := upper(replace(replace(btrim(size),',','.'),' ',''));
 m := regexp_match(s,'^([0-9]{3})/([0-9]{2,3})R([0-9]{2}(\.[0-9]+)?)$');
 IF m IS NOT NULL AND m[1]::numeric>0 AND m[2]::numeric BETWEEN 1 AND 100 AND m[3]::numeric>0 THEN
  RETURN jsonb_build_object('width',m[1]::numeric,'unit','mm','profile',m[2]::numeric,'rim',m[3]::numeric);
 END IF;
 m := regexp_match(s,'^([0-9]{3})R([0-9]{2}(\.[0-9]+)?)$');
 IF m IS NOT NULL AND m[1]::numeric>0 AND m[2]::numeric>0 THEN
  RETURN jsonb_build_object('width',m[1]::numeric,'unit','mm','rim',m[2]::numeric);
 END IF;
 m := regexp_match(s,'^([0-9]{1,2}(\.[0-9]+)?)R([0-9]{2}(\.[0-9]+)?)$');
 IF m IS NOT NULL AND m[1]::numeric>0 AND m[3]::numeric>0 THEN
  RETURN jsonb_build_object('width',m[1]::numeric,'unit','in','rim',m[3]::numeric);
 END IF;
 RETURN '{}'::jsonb;
END $$;
-- +goose StatementEnd

ALTER TABLE catalog.products ADD CONSTRAINT products_id_kind_unique UNIQUE(product_id,kind);
CREATE TABLE catalog.tyre_specs (
 product_id uuid PRIMARY KEY,
 kind text NOT NULL DEFAULT 'tyre' CHECK(kind='tyre'),
 FOREIGN KEY(product_id,kind) REFERENCES catalog.products(product_id,kind) ON DELETE CASCADE,
 size text NOT NULL DEFAULT '',
 season text NOT NULL DEFAULT '',
 tread_percent numeric CHECK(tread_percent BETWEEN 0 AND 100),
 load_index text NOT NULL DEFAULT '',
 speed_index text NOT NULL DEFAULT '',
 width_value numeric GENERATED ALWAYS AS ((catalog.tyre_dimensions(size)->>'width')::numeric) STORED,
 width_unit text GENERATED ALWAYS AS (catalog.tyre_dimensions(size)->>'unit') STORED,
 aspect_ratio numeric GENERATED ALWAYS AS ((catalog.tyre_dimensions(size)->>'profile')::numeric) STORED,
 rim_diameter_inches numeric GENERATED ALWAYS AS ((catalog.tyre_dimensions(size)->>'rim')::numeric) STORED
);
CREATE TABLE catalog.wheel_specs (
 product_id uuid PRIMARY KEY,
 kind text NOT NULL DEFAULT 'wheel' CHECK(kind='wheel'),
 FOREIGN KEY(product_id,kind) REFERENCES catalog.products(product_id,kind) ON DELETE CASCADE,
 marking text NOT NULL DEFAULT '',
 diameter_inches numeric CHECK(diameter_inches>0 AND diameter_inches<=10000),
 width_inches numeric CHECK(width_inches>0 AND width_inches<=10000),
 offset_mm numeric,
 pcd text NOT NULL DEFAULT '',
 wheel_type text NOT NULL DEFAULT '',
 center_bore_mm numeric CHECK(center_bore_mm>0 AND center_bore_mm<=10000)
);
INSERT INTO catalog.tyre_specs(product_id,size,season,tread_percent,load_index,speed_index)
 SELECT product_id,size,season,tread_percent,load_index,speed_index FROM catalog.products WHERE kind='tyre';
INSERT INTO catalog.wheel_specs(product_id,marking,diameter_inches,width_inches,offset_mm,pcd,wheel_type,center_bore_mm)
 SELECT product_id,size,diameter,width,wheel_offset,pcd,wheel_type,center_bore FROM catalog.products WHERE kind='wheel';
ALTER TABLE catalog.products DROP COLUMN size,DROP COLUMN season,DROP COLUMN tread_percent,
 DROP COLUMN load_index,DROP COLUMN speed_index,DROP COLUMN diameter,DROP COLUMN width,
 DROP COLUMN wheel_offset,DROP COLUMN pcd,DROP COLUMN wheel_type,DROP COLUMN center_bore;

-- Read-only projection preserves the catalog's flat HTTP details contract.
CREATE VIEW catalog.product_details AS
 SELECT p.*,coalesce(t.size,w.marking,'') AS size,coalesce(t.season,'') AS season,
 t.tread_percent,coalesce(t.load_index,'') AS load_index,coalesce(t.speed_index,'') AS speed_index,
 w.diameter_inches AS diameter,w.width_inches AS width,w.offset_mm AS wheel_offset,
 coalesce(w.pcd,'') AS pcd,coalesce(w.wheel_type,'') AS wheel_type,w.center_bore_mm AS center_bore,
 t.width_value AS tyre_width,coalesce(t.width_unit,'') AS tyre_width_unit,
 t.aspect_ratio,t.rim_diameter_inches AS rim_diameter
 FROM catalog.products p LEFT JOIN catalog.tyre_specs t USING(product_id)
 LEFT JOIN catalog.wheel_specs w USING(product_id);
-- +goose StatementBegin
DO $$ BEGIN
 EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON catalog.tyre_specs,catalog.wheel_specs TO %I',current_database() || '_catalog');
 EXECUTE format('GRANT SELECT ON catalog.product_details TO %I',current_database() || '_catalog');
END $$;
-- +goose StatementEnd
