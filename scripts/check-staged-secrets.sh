#!/usr/bin/env bash
set -euo pipefail

# Run inside Linux. Never pass secret values as command arguments or print matches.
password_file=${PGPASSWORD_FILE:-/run/secrets/postgres_password}
if [[ ! -r "$password_file" || ! -s "$password_file" ]]; then
  echo 'Secret check requires the local PostgreSQL secret; check Compose mounts.' >&2
  exit 1
fi
if ! LC_ALL=C rg --quiet '[^[:space:]]' "$password_file"; then
  echo 'Secret check requires a nonempty PostgreSQL secret.' >&2
  exit 1
fi

umask 077
diff_file=$(mktemp)
trap 'rm -f "$diff_file"' EXIT
git diff --cached --no-ext-diff --no-textconv --binary > "$diff_file"
# Ignore empty lines in a secret file, which would otherwise match every line.
if rg --fixed-strings --quiet --file <(sed '/^$/d' "$password_file") "$diff_file"; then
  echo 'Commit blocked: staged diff contains the local PostgreSQL secret.' >&2
  exit 1
else
  result=$?
  if [[ "$result" != 1 ]]; then
    echo 'Secret check failed to inspect the staged diff.' >&2
    exit "$result"
  fi
fi
