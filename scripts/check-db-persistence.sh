#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
export PGHOST=postgres PGPORT=5432 PGUSER=tireshop
# PGPASSWORD_FILE is our convention; libpq itself does not read it.
password_file=${PGPASSWORD_FILE:-/run/secrets/postgres_password}
if [[ ! -r "$password_file" || ! -s "$password_file" ]]; then
  echo 'PostgreSQL secret is missing, empty or unreadable; check Compose secrets.' >&2
  exit 1
fi
export PGPASSWORD
PGPASSWORD=$(< "$password_file")
if [[ -z "$PGPASSWORD" ]]; then
  echo 'PostgreSQL secret is empty.' >&2
  exit 1
fi
export PGCONNECT_TIMEOUT=5
export PGDATABASE=tireshop_envcheck
state_file=.tmp/db-persistence-state

query() {
  psql -X -v ON_ERROR_STOP=1 -At "$@"
}

case "${1:-}" in
  prepare)
    if [[ -e "$state_file" ]]; then
      echo 'Probe already prepared; run verify after restarting postgres.' >&2
      exit 1
    fi
    exists=$(query -d postgres -c "SELECT 1 FROM pg_database WHERE datname = 'tireshop_envcheck'")
    if [[ "$exists" != 1 ]]; then
      createdb tireshop_envcheck
    fi
    marker=$(cat /proc/sys/kernel/random/uuid)
    started=$(query -c 'SELECT pg_postmaster_start_time()')
    query -v marker="$marker" <<'SQL'
BEGIN;
CREATE TABLE IF NOT EXISTS persistence_probe (
    marker uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO persistence_probe (marker) VALUES (:'marker');
COMMIT;
SQL
    mkdir -p .tmp
    (umask 077; printf '%s\n%s\n' "$marker" "$started" > "$state_file")
    echo 'Probe committed in tireshop_envcheck. Restart postgres from the host, then run verify.'
    ;;
  verify)
    if [[ ! -f "$state_file" ]]; then
      echo 'Run prepare first.' >&2
      exit 1
    fi
    mapfile -t state < "$state_file"
    marker=${state[0]}
    started=$(query -c 'SELECT pg_postmaster_start_time()')
    if [[ "$started" == "${state[1]}" ]]; then
      echo 'PostgreSQL has not restarted; persistence is not verified.' >&2
      exit 1
    fi
    found=$(query -v marker="$marker" <<'SQL'
SELECT count(*) FROM persistence_probe WHERE marker = :'marker'::uuid;
SQL
    )
    if [[ "$found" != 1 ]]; then
      echo 'Probe missing after restart.' >&2
      exit 1
    fi
    echo 'PASS: PostgreSQL restarted and the committed probe survived.'
    ;;
  *)
    echo 'Usage: bash scripts/check-db-persistence.sh prepare|verify' >&2
    exit 2
    ;;
esac
