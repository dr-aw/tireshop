-- +goose Up
ALTER TABLE catalog.products
 ADD COLUMN sku text NOT NULL DEFAULT '',
 ADD COLUMN kind text NOT NULL DEFAULT 'tyre' CHECK (kind IN ('tyre','wheel')),
 ADD COLUMN description text NOT NULL DEFAULT '',
 ADD COLUMN brand text NOT NULL DEFAULT '',
 ADD COLUMN model text NOT NULL DEFAULT '',
 ADD COLUMN production_year integer CHECK (production_year BETWEEN 1900 AND 2200),
 ADD COLUMN size text NOT NULL DEFAULT '',
 ADD COLUMN season text NOT NULL DEFAULT '',
 ADD COLUMN tread_percent numeric CHECK (tread_percent BETWEEN 0 AND 100),
 ADD COLUMN load_index text NOT NULL DEFAULT '',
 ADD COLUMN speed_index text NOT NULL DEFAULT '',
 ADD COLUMN location text NOT NULL DEFAULT '',
 ADD COLUMN condition text NOT NULL DEFAULT '',
 ADD COLUMN diameter numeric CHECK (diameter > 0),
 ADD COLUMN width numeric CHECK (width > 0),
 ADD COLUMN wheel_offset numeric,
 ADD COLUMN pcd text NOT NULL DEFAULT '',
 ADD COLUMN wheel_type text NOT NULL DEFAULT '',
 ADD COLUMN center_bore numeric CHECK (center_bore > 0),
 ADD COLUMN units_per_sale integer NOT NULL DEFAULT 1 CHECK (units_per_sale BETWEEN 1 AND 100),
 ADD COLUMN review_notes text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX products_sku_unique ON catalog.products(sku) WHERE sku <> '';

CREATE TABLE catalog.import_batches (
 source_hash text PRIMARY KEY,
 source_workbooks jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 product_count integer NOT NULL
);
CREATE TABLE catalog.source_rows (
 product_id uuid PRIMARY KEY REFERENCES catalog.products(product_id),
 source_hash text NOT NULL REFERENCES catalog.import_batches(source_hash),
 source_file text NOT NULL,
 sheet_name text NOT NULL,
 row_number integer NOT NULL,
 raw_cells jsonb NOT NULL
);
CREATE TABLE catalog.product_images (
 product_id uuid NOT NULL REFERENCES catalog.products(product_id),
 position integer NOT NULL CHECK (position >= 0),
 source_path text NOT NULL,
 source_sha256 text NOT NULL,
 webp_path text NOT NULL,
 width integer NOT NULL CHECK (width > 0),
 height integer NOT NULL CHECK (height > 0),
 PRIMARY KEY(product_id, position),
 UNIQUE(product_id, source_sha256)
);
CREATE OR REPLACE VIEW catalog.request_products_v1 AS
 SELECT product_id,name,unit_price_rub,is_available,
 CASE WHEN units_per_sale=1 THEN 'шт.' ELSE 'комплект (' || units_per_sale || ' шт.)' END AS unit_label
 FROM catalog.products;
-- +goose StatementBegin
DO $$ BEGIN
 EXECUTE format('GRANT SELECT, INSERT ON catalog.import_batches, catalog.source_rows TO %I',current_database() || '_catalog');
 EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON catalog.product_images TO %I',current_database() || '_catalog');
END $$;
-- +goose StatementEnd
