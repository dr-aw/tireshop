#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ ! "${PGDATABASE:-}" =~ ^tireshop_test_[a-f0-9]{16}$ ]]; then
  echo 'Requires isolated test DB.' >&2; exit 1
fi
source scripts/local-db-env.sh
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="host=postgres port=5432 user=tireshop dbname=$PGDATABASE sslmode=disable options='-c role=${PGDATABASE}_migrator'"
psql -X -v ON_ERROR_STOP=1 <<'SQL'
WITH p AS (INSERT INTO catalog.products(name,kind,unit_price_rub,stock) VALUES ('Diameter migration tyre','tyre',100,7) RETURNING product_id)
INSERT INTO catalog.tyre_specs(product_id,size) SELECT product_id,'205/85R16' FROM p;
WITH p AS (INSERT INTO catalog.products(name,kind,unit_price_rub,stock) VALUES ('Diameter migration wheel','wheel',200,2) RETURNING product_id)
INSERT INTO catalog.wheel_specs(product_id,diameter_inches,width_inches) SELECT product_id,17.5,6.75 FROM p;
CREATE TABLE public.diameter_before AS SELECT to_jsonb(p) AS value FROM catalog.product_details p;
CREATE TABLE public.diameter_indexes_before AS SELECT indexrelid FROM pg_index WHERE indexrelid IN ('catalog.tyre_specs_selection_idx'::regclass,'catalog.wheel_specs_selection_idx'::regclass);
SQL
mise exec -- goose -env none -dir migrations/catalog -table catalog.goose_db_version up-to 6
psql -X -v ON_ERROR_STOP=1 <<'SQL'
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.diameter_before b FULL JOIN catalog.product_details p ON p.product_id=(b.value->>'product_id')::uuid WHERE p.product_id IS NULL OR b.value IS NULL OR to_jsonb(p)-'diameter' <> b.value-'diameter' OR p.diameter IS DISTINCT FROM (b.value->>'radius')::numeric) THEN
 RAISE EXCEPTION 'Data changed during rename'; END IF;
 IF (SELECT count(*) FROM public.diameter_indexes_before b JOIN pg_index i USING(indexrelid))<>2 THEN RAISE EXCEPTION 'Indexes replaced/lost'; END IF;
END $$;
-- Deliberately invalid v6 data must prevent v7, not be silently rewritten.
UPDATE catalog.wheel_specs SET diameter_inches=26;
SQL
if mise exec -- goose -env none -dir migrations/catalog -table catalog.goose_db_version up-to 7 > /dev/null 2>&1; then
  echo 'Expected constraint migration to reject existing invalid diameter.' >&2; exit 1
fi
psql -X -v ON_ERROR_STOP=1 <<'SQL'
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_constraint WHERE conname IN ('tyre_specs_diameter_range','wheel_specs_diameter_range')) THEN RAISE EXCEPTION 'Failed migration was not atomic'; END IF;
END $$;
UPDATE catalog.wheel_specs SET diameter_inches=17.5;
SQL
mise exec -- goose -env none -dir migrations/catalog -table catalog.goose_db_version up-to 7
psql -X -v ON_ERROR_STOP=1 <<'SQL'
DELETE FROM catalog.products WHERE name LIKE 'Diameter migration %';
DROP TABLE public.diameter_before,public.diameter_indexes_before;
SQL
echo 'PASS: 5→6 preserves data/indexes; 7 rejects invalid existing data atomically.'
