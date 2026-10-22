#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/local-db-env.sh
export GOOSE_DRIVER=postgres
# Password comes from the secret via the process environment, never argv/DSN.
export GOOSE_DBSTRING="host=postgres port=5432 user=tireshop dbname=$PGDATABASE sslmode=disable options='-c role=${PGDATABASE}_migrator'"
for service in catalog requests; do
  mise exec -- goose -env none -dir "migrations/$service" -table "$service.goose_db_version" up
done
