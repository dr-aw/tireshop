#!/usr/bin/env bash
# Run on the HOST from the repository root, before Compose mounts these files.
# Existing secrets are retained; rotating a password is an explicit operation.
set -euo pipefail
secret_dir=${1:-../tireshop-secrets}
mkdir -p "$secret_dir"
chmod 700 "$secret_dir"
for service in catalog requests; do
  secret_path="$secret_dir/${service}_password"
  if [[ ! -e "$secret_path" ]]; then
    (umask 077; set -C; openssl rand -hex 32 > "$secret_path")
  fi
  chmod 600 "$secret_path"
done
echo 'Runtime secret files are ready. Values were not printed.'
