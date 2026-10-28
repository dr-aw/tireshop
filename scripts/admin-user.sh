#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/local-db-env.sh
mise exec -- go run ./cmd/admin-user "$@"
