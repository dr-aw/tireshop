#!/usr/bin/env bash
# Runs only in the ephemeral Linux Actions job container, never against Aiven.
set -euo pipefail
: "${GITHUB_ENV:?Actions environment file required}"
: "${GITHUB_PATH:?Actions path file required}"
: "${RUNNER_TEMP:?Actions temporary directory required}"
# Single source for mise itself: the existing development image definition.
ci_mise_version=$(sed -n 's/^ARG MISE_VERSION=//p' .devcontainer/Dockerfile)
[[ "$ci_mise_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
curl -fsSL https://mise.run -o "$RUNNER_TEMP/install-mise.sh"
MISE_VERSION="$ci_mise_version" MISE_INSTALL_PATH=/usr/local/bin/mise sh "$RUNNER_TEMP/install-mise.sh"
mise trust "$PWD/mise.toml"
mise install --locked
printf '%s\n' "$HOME/.local/share/mise/shims" >> "$GITHUB_PATH"
mise reshim
umask 077
ci_password_file="$RUNNER_TEMP/ci-postgres-password"
od -An -N32 -tx1 /dev/urandom | tr -d ' \n' > "$ci_password_file"
printf 'PGPASSWORD_FILE=%s\n' "$ci_password_file" >> "$GITHUB_ENV"
