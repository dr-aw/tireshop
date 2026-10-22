#!/usr/bin/env bash
# Source only from local DB commands. Never enable shell tracing around secrets.
set -euo pipefail
export PGHOST=postgres PGPORT=5432 PGUSER=tireshop PGCONNECT_TIMEOUT=5 PGSSLMODE=disable
unset PGHOSTADDR PGSERVICE PGSERVICEFILE PGOPTIONS
export PGDATABASE=${PGDATABASE:-tireshop_dev}
if [[ "$PGDATABASE" != tireshop_dev && ! "$PGDATABASE" =~ ^tireshop_test_[a-f0-9]{16}$ ]]; then
  echo 'Only tireshop_dev or an isolated tireshop_test_<16 hex> database is allowed.' >&2
  exit 1
fi
local_db_password_file=${PGPASSWORD_FILE:-/run/secrets/postgres_password}
if [[ ! -r "$local_db_password_file" || ! -s "$local_db_password_file" ]]; then
  echo 'Local PostgreSQL secret is missing or unreadable.' >&2
  exit 1
fi
export PGPASSWORD
PGPASSWORD=$(< "$local_db_password_file")
if [[ -z "$PGPASSWORD" ]]; then
  echo 'Local PostgreSQL secret is empty.' >&2
  exit 1
fi
