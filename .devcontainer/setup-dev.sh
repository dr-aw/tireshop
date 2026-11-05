#!/usr/bin/env bash
set -euo pipefail

cd /workspace
mise trust /workspace/mise.toml
mise install --locked
mise reshim
git config --local core.hooksPath .githooks

# Check the same stable entry points available to editor extensions.
"${MISE_DATA_DIR:-$HOME/.local/share/mise}/shims/go" version
"${MISE_DATA_DIR:-$HOME/.local/share/mise}/shims/goose" -version
