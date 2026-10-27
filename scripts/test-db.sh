#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Always create a NEW database; never reset a caller-supplied database.
export PGDATABASE="tireshop_test_$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')"
source scripts/local-db-env.sh
created=false
cleanup() {
  local test_exit=$?
  trap - EXIT
  if [[ "$created" == true ]]; then
    if ! dropdb --if-exists "$PGDATABASE"; then
      echo 'Temporary database cleanup failed; do not drop another database.' >&2
      exit 1
    fi
    if ! psql -X -d postgres -v ON_ERROR_STOP=1 -v database="$PGDATABASE" <<'SQL'
SELECT format('DROP ROLE IF EXISTS %I', :'database' || suffix)
FROM (VALUES ('_requests'), ('_catalog'), ('_migrator')) AS roles(suffix)
\gexec
SQL
    then
      echo 'Temporary role cleanup failed.' >&2
      exit 1
    fi
  fi
  exit "$test_exit"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
createdb "$PGDATABASE"
created=true
bash scripts/db-bootstrap.sh
bash scripts/db-migrate.sh
# Run up again to check repeatability, without dropping data.
bash scripts/db-migrate.sh
export TIRESHOP_TEST_DB="$PGDATABASE"
mise exec -- go test -race -count=1 -p 1 -tags=integration ./internal/requests/postgres ./internal/httpapi
# Confirm that up also preserves fixtures and requests written by the tests.
before_count=$(psql -X -At -v ON_ERROR_STOP=1 -c 'SELECT count(*) FROM requests.requests')
bash scripts/db-migrate.sh
after_count=$(psql -X -At -v ON_ERROR_STOP=1 -c 'SELECT count(*) FROM requests.requests')
test "$before_count" = "$after_count"
echo 'PASS: migrations preserve existing test requests.'
