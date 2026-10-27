#!/usr/bin/env bash
# Synthetic fixtures belong to development only, never to schema migrations.
set -euo pipefail
cd "$(dirname "$0")/.."
export PGHOST=postgres PGPORT=5432 PGDATABASE=tireshop_dev PGUSER=tireshop_dev_catalog_login PGSSLMODE=disable PGCONNECT_TIMEOUT=5
unset PGHOSTADDR PGSERVICE PGSERVICEFILE PGOPTIONS
password_file=/run/secrets/catalog_password
if [[ ! -r "$password_file" || ! -s "$password_file" ]]; then
  echo 'Catalog Compose secret is missing.' >&2
  exit 1
fi
export PGPASSWORD
PGPASSWORD=$(< "$password_file")
psql -X -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO catalog.products (product_id, name, unit_price_rub, is_available) VALUES
('11111111-1111-4111-8111-111111111111', 'Шина 315/70 R22.5 — демонстрационный товар', 18000, true),
('22222222-2222-4222-8222-222222222222', 'Шина 385/65 R22.5 — демонстрационный товар', 22500, true),
('33333333-3333-4333-8333-333333333333', 'Диск 22.5 × 9.00 — демонстрационный товар', 9500, true)
ON CONFLICT (product_id) DO NOTHING;
SQL
