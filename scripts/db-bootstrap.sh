#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/local-db-env.sh
# Only administrative provisioning: roles and empty namespaces for goose's
# version tables. Application tables/views are created exclusively by migrations.
psql -X -v ON_ERROR_STOP=1 -v database="$PGDATABASE" <<'SQL'
SELECT format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT', :'database' || suffix)
FROM (VALUES ('_migrator'), ('_catalog'), ('_requests')) AS roles(suffix)
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'database' || suffix)
\gexec
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', :'database')
\gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO %I, %I, %I',
  :'database', :'database' || '_migrator', :'database' || '_catalog', :'database' || '_requests')
\gexec
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
SELECT format('CREATE SCHEMA IF NOT EXISTS %I AUTHORIZATION %I', schema_name, :'database' || '_migrator')
FROM (VALUES ('catalog'), ('requests')) AS schemas(schema_name)
\gexec
SQL
