#!/usr/bin/env bash
# All data and roles are synthetic; target is fixed local postgres by the guard.
set -euo pipefail
cd "$(dirname "$0")/.."
export PGDATABASE="tireshop_test_$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')"
source scripts/local-db-env.sh
operator_role="${PGDATABASE}_operator"
cleanup() {
  local result=$?
  trap - EXIT
  dropdb --if-exists "$PGDATABASE"
  psql -X -q -d postgres -v ON_ERROR_STOP=1 -v db="$PGDATABASE" <<'SQL'
SELECT format('DROP ROLE IF EXISTS %I', :'db' || suffix)
FROM (VALUES ('_catalog_login'),('_requests_login'),('_migration_login'),('_catalog'),('_requests'),('_migrator'),('_operator')) AS r(suffix)
\gexec
SQL
  exit "$result"
}
trap cleanup EXIT
psql -X -q -d postgres -v ON_ERROR_STOP=1 -v operator="$operator_role" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN NOINHERIT CREATEROLE CREATEDB NOSUPERUSER', :'operator')
\gexec
SQL
createdb --owner="$operator_role" "$PGDATABASE"
# SET SESSION AUTHORIZATION reproduces permission checks for a non-superuser
# operator without giving it a password or touching any existing user.
psql -X -q -v ON_ERROR_STOP=1 -v operator="$operator_role" <<'SQL'
SELECT format('SET SESSION AUTHORIZATION %I', :'operator')
\gexec
\i deploy/aiven/bootstrap.sql
SQL
# Verify the one-shot bootstrap refuses a repeat without changing existing data.
if psql -X -q -v ON_ERROR_STOP=1 -f deploy/aiven/bootstrap.sql >/dev/null 2>&1; then
  echo 'Repeated bootstrap must fail.' >&2; exit 1
fi
migration_password=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
if ! printf '%s\n%s\n' "$migration_password" "$migration_password" |
  psql -X -q -v ON_ERROR_STOP=1 -c "\\password ${PGDATABASE}_migration_login" >/dev/null 2>&1; then
  echo 'Could not configure synthetic migration login.' >&2; exit 1
fi
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="host=postgres port=5432 user=${PGDATABASE}_migration_login dbname=$PGDATABASE sslmode=disable options='-c role=${PGDATABASE}_migrator'"
for service in catalog requests; do
  PGPASSWORD="$migration_password" mise exec -- goose -env none -dir "migrations/$service" -table "$service.goose_db_version" up
  PGPASSWORD="$migration_password" mise exec -- goose -env none -dir "migrations/$service" -table "$service.goose_db_version" up
done
psql -X -q -v ON_ERROR_STOP=1 <<'SQL'
DO $checks$
DECLARE db text := current_database();
BEGIN
 IF has_database_privilege(db || '_catalog_login', db, 'CREATE')
 OR has_schema_privilege(db || '_requests_login', 'catalog', 'CREATE')
 OR has_table_privilege(db || '_requests_login', 'catalog.products', 'UPDATE')
 OR has_table_privilege(db || '_catalog_login', 'requests.requests', 'SELECT')
 OR pg_has_role(db || '_requests_login', db || '_migrator','MEMBER')
 THEN RAISE EXCEPTION 'Runtime permissions too broad'; END IF;
 IF NOT has_table_privilege(db || '_catalog_login','catalog.products','UPDATE')
 OR NOT has_table_privilege(db || '_requests_login','requests.requests','INSERT')
 OR NOT has_table_privilege(db || '_requests_login','catalog.request_products_v1','SELECT')
 OR NOT pg_has_role(db || '_migration_login', db || '_migrator','SET')
 THEN RAISE EXCEPTION 'Required runtime/migration permissions missing'; END IF;
END
$checks$;
SQL
# Run goose through the same SET ROLE route a non-superuser migration LOGIN uses.
# The local synthetic superuser connects, then session authorization removes its
# powers before switching to the migration owner for DDL.
psql -X -q -v ON_ERROR_STOP=1 -v db="$PGDATABASE" <<'SQL'
SELECT format('SET SESSION AUTHORIZATION %I', :'db' || '_migration_login')
\gexec
SELECT format('SET ROLE %I', :'db' || '_migrator')
\gexec
CREATE TABLE catalog.bootstrap_permission_probe(id int);
DROP TABLE catalog.bootstrap_permission_probe;
SQL
echo 'PASS: non-superuser bootstrap, migrations, repeated up and restricted permissions.'
