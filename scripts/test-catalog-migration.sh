#!/usr/bin/env bash
# Called only with the disposable database created by test-db.sh.
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ ! "${PGDATABASE:-}" =~ ^tireshop_test_[a-f0-9]{16}$ ]]; then
  echo 'Migration verification requires an isolated test database.' >&2
  exit 1
fi
source scripts/local-db-env.sh
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="host=postgres port=5432 user=tireshop dbname=$PGDATABASE sslmode=disable options='-c role=${PGDATABASE}_migrator'"
mise exec -- goose -env none -dir migrations/catalog -table catalog.goose_db_version up-to 3
psql -X -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO catalog.products(product_id,name,sku,kind,size,stock,version,unit_price_rub,units_per_sale,diameter,width,wheel_offset,pcd,center_bore,season,tread_percent) VALUES
 ('eeeeeeee-0000-4000-8000-000000000001','Migration metric','MIGRATE-METRIC','tyre','295/80R22.5',7,9,12345,1,NULL,NULL,NULL,'',NULL,'Лето',75),
 ('eeeeeeee-0000-4000-8000-000000000002','Migration inches','MIGRATE-INCH','tyre','11R22.5',4,5,15000,1,NULL,NULL,NULL,'',NULL,'',NULL),
 ('eeeeeeee-0000-4000-8000-000000000003','Migration wheel','MIGRATE-WHEEL','wheel','Original wheel marking',1,3,25000,2,22.5,8.25,-118.5,'10x335',281,NULLIF('','x'),NULL),
 ('eeeeeeee-0000-4000-8000-000000000004','Migration unknown','MIGRATE-UNKNOWN','tyre','Unrecognized original',0,1,100,1,NULL,NULL,NULL,'',NULL,'',NULL);
CREATE TABLE public.catalog_migration_before AS SELECT to_jsonb(p) AS value FROM catalog.products p;
INSERT INTO catalog.product_images(product_id,position,source_path,source_sha256,webp_path,width,height)
 VALUES('eeeeeeee-0000-4000-8000-000000000001',0,'synthetic.jpg','synthetic','/media/synthetic.webp',1280,1280);
SQL
mise exec -- goose -env none -dir migrations/catalog -table catalog.goose_db_version up
psql -X -v ON_ERROR_STOP=1 <<'SQL'
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.catalog_migration_before b
 LEFT JOIN catalog.product_details p ON p.product_id=(b.value->>'product_id')::uuid
 WHERE p.product_id IS NULL OR to_jsonb(p)-ARRAY['tyre_width','tyre_width_unit','aspect_ratio','rim_diameter'] <> b.value) THEN
  RAISE EXCEPTION 'Existing values changed during migration';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM catalog.tyre_specs WHERE size='295/80R22.5' AND width_value=295 AND width_unit='mm' AND aspect_ratio=80 AND rim_diameter_inches=22.5)
 OR NOT EXISTS(SELECT 1 FROM catalog.tyre_specs WHERE size='11R22.5' AND width_value=11 AND width_unit='in' AND aspect_ratio IS NULL AND rim_diameter_inches=22.5)
 OR NOT EXISTS(SELECT 1 FROM catalog.tyre_specs WHERE size='Unrecognized original' AND width_value IS NULL AND aspect_ratio IS NULL AND rim_diameter_inches IS NULL)
 THEN RAISE EXCEPTION 'Wrong tyre dimensions'; END IF;
 IF (SELECT count(*) FROM catalog.specs_migration_archive)<>4 OR (SELECT count(*) FROM catalog.product_images)<>1 THEN RAISE EXCEPTION 'Archive/images lost'; END IF;
 IF NOT EXISTS(SELECT 1 FROM catalog.request_products_v1 WHERE product_id='eeeeeeee-0000-4000-8000-000000000003' AND unit_label='комплект (2 шт.)') THEN RAISE EXCEPTION 'Request contract changed'; END IF;
END $$;
DELETE FROM catalog.product_images;
DELETE FROM catalog.products WHERE sku LIKE 'MIGRATE-%';
DELETE FROM catalog.specs_migration_archive;
DROP TABLE public.catalog_migration_before;
SQL
echo 'PASS: populated catalog migration preserves values, units, stock, versions and media.'
