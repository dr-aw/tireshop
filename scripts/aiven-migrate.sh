#!/usr/bin/env bash
# Operator-only runner; never source local-db-env.sh or run from CI/dev/agent.
set -euo pipefail
cd "$(dirname "$0")/.."
command=${1:-status}
case "$command" in
  status|version) [[ $# -le 1 ]] ;;
  up-to)
    [[ $# -eq 3 && "$2" =~ ^[0-9]+$ && "$3" =~ ^[0-9]+$ ]] || {
      echo 'Usage: aiven-migrate.sh up-to <catalog-version> <requests-version>' >&2; exit 1;
    } ;;
  *) echo 'Only status, version or up-to are supported; automatic down is prohibited.' >&2; exit 1 ;;
esac
# Fixed target prevents a local script or inherited PG environment choosing a DB.
export PGHOST=pg-2f7b3aad-snorchello-5783.g.aivencloud.com PGPORT=22201 PGDATABASE=defaultdb
export PGUSER=defaultdb_migration_login PGSSLMODE=verify-full PGCONNECT_TIMEOUT=10
unset PGHOSTADDR PGSERVICE PGSERVICEFILE PGOPTIONS PGPASSFILE PGPASSWORD DATABASE_URL GOOSE_DBSTRING GOOSE_DRIVER
: "${PGSSLROOTCERT:?Set the absolute path to the downloaded Aiven CA}"
: "${MIGRATION_PASSWORD_FILE:?Set the absolute path to the migration login password file}"
[[ "$PGSSLROOTCERT" == /* && -s "$PGSSLROOTCERT" && "$MIGRATION_PASSWORD_FILE" == /* && -s "$MIGRATION_PASSWORD_FILE" ]]
# libpq does not understand *_PASSWORD_FILE. Only this process and its children
# receive the file's value, never the shell history, DSN, argv or build environment.
export PGPASSWORD
PGPASSWORD=$(< "$MIGRATION_PASSWORD_FILE")
[[ -n "$PGPASSWORD" ]]
trap 'unset PGPASSWORD' EXIT
# URI carries only non-secret SSL options; DB host/user are explicit and fixed.
# Escape the local CA path for the libpq conninfo quoted string.
ca_path=${PGSSLROOTCERT//\\/\\\\}
ca_path=${ca_path//\'/\\\'}
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="host=$PGHOST port=$PGPORT dbname=$PGDATABASE user=$PGUSER sslmode=verify-full sslrootcert='$ca_path' options='-c role=defaultdb_migrator' connect_timeout=10"
for service in catalog requests; do
  args=("$command")
  if [[ "$command" == up-to ]]; then
    if [[ "$service" == catalog ]]; then args+=("$2"); else args+=("$3"); fi
  fi
  # Diagnostic output never includes connection strings. Capture provider errors:
  # they can contain server-provided text, so don't print them automatically.
  migration_log=$(mktemp)
  if ! mise exec -- goose -env none -dir "migrations/$service" -table "$service.goose_db_version" "${args[@]}" >"$migration_log" 2>&1; then
    echo "Migration $command failed for $service. Private diagnostic file: $migration_log" >&2
    exit 1
  fi
  echo "PASS: $service $command. Private diagnostic file: $migration_log"
done
