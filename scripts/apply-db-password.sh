#!/usr/bin/env bash
set -euo pipefail

# Run via stdin in the postgres container, using its local Unix socket.
# POSTGRES_PASSWORD_FILE only initializes a new cluster; existing roles need ALTER.
password_file=${POSTGRES_PASSWORD_FILE:-/run/secrets/postgres_password}
if [[ ! -r "$password_file" || ! -s "$password_file" ]]; then
  echo 'PostgreSQL secret is missing, empty or unreadable.' >&2
  exit 1
fi
export PGPASSWORD
PGPASSWORD=$(< "$password_file")
if [[ -z "$PGPASSWORD" ]]; then
  echo 'PostgreSQL secret is empty.' >&2
  exit 1
fi
psql -X -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-tireshop}" -d postgres <<'SQL'
\getenv db_password PGPASSWORD
\getenv db_user POSTGRES_USER
SET log_statement = 'none';
SET log_min_error_statement = 'panic';
ALTER ROLE :"db_user" PASSWORD :'db_password';
SQL
echo 'Local PostgreSQL role password updated.'
