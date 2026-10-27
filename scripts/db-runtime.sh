#!/usr/bin/env bash
# Provision separate LOGIN accounts; API processes never receive the admin
# password. Run inside dev after mounting the two runtime Compose secrets.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/local-db-env.sh
for service in catalog requests; do
  password_file="/run/secrets/${service}_password"
  if [[ ! -r "$password_file" || ! -s "$password_file" ]]; then
    echo 'Runtime Compose secrets are missing. Create/mount them from the host first.' >&2
    exit 1
  fi
done
for service in catalog requests; do
  runtime_role="${PGDATABASE}_${service}_login"
  psql -X -q -v ON_ERROR_STOP=1 -v login="$runtime_role" -v runtime="${PGDATABASE}_${service}" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE', :'login')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname=:'login')
\gexec
SELECT format('ALTER ROLE %I LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE', :'login')
\gexec
SELECT format('GRANT %I TO %I', :'runtime', :'login')
\gexec
SQL
  runtime_password=$(< "/run/secrets/${service}_password")
  # psql's password command hashes on the client. Passwords arrive on stdin,
  # never in argv/SQL source files. Suppress prompts and diagnostics containing
  # credential material; failures are reported with a generic message.
  if ! printf '%s\n%s\n' "$runtime_password" "$runtime_password" |
    psql -X -q -v ON_ERROR_STOP=1 -c "\\password $runtime_role" >/dev/null 2>&1; then
    echo 'Could not set runtime password.' >&2
    exit 1
  fi
  unset runtime_password
done
echo 'Runtime database logins are ready.'
