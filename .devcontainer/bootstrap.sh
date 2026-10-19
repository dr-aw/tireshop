#!/usr/bin/env bash
set -euo pipefail

cd /workspace
rm -f /tmp/tireshop-bootstrap-ready
test -f mise.toml
test -f mise.lock

# This entrypoint is for this trusted project. It never regenerates the lockfile.
mise trust /workspace/mise.toml
mise install --locked
mise reshim
mise exec -- codex --version

chmod 700 /home/vscode/.codex
touch /tmp/tireshop-bootstrap-ready
echo 'tireshop: bootstrap complete'
exec "$@"
