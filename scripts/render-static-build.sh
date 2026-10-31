#!/usr/bin/env bash
# Render Static Site build only. No DB credentials belong in this environment.
set -euo pipefail
cd "$(dirname "$0")/.."
render_mise_version=$(sed -n 's/^ARG MISE_VERSION=//p' .devcontainer/Dockerfile)
[[ "$render_mise_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
render_install_script=$(mktemp)
trap 'rm -f "$render_install_script"' EXIT
curl -fsSL https://mise.run -o "$render_install_script"
MISE_VERSION="$render_mise_version" MISE_INSTALL_PATH="$HOME/.local/bin/mise" sh "$render_install_script"
export PATH="$HOME/.local/bin:$PATH"
mise trust "$PWD/mise.toml"
mise install --locked
mise run frontend:install
mise run frontend:build
