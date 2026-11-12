#!/usr/bin/env bash
# Timeweb build: public snapshot only, no application credentials.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${SSG_CATALOG_URL:?Set the public catalog /api/products/snapshot URL}"
: "${SITE_ORIGIN:?Set the public site HTTPS origin}"
if [[ "$SSG_CATALOG_URL" != https://* || "$SITE_ORIGIN" != https://* ]]; then
  echo 'Timeweb publication requires HTTPS snapshot and site URLs.' >&2
  exit 1
fi
if [[ -n "${SSG_SNAPSHOT_FILE:-}" ]]; then
  echo 'Timeweb publication requires the public API snapshot, not a local file.' >&2
  exit 1
fi
export MISE_ENV=
timeweb_mise_version=$(sed -n 's/^ARG MISE_VERSION=//p' .devcontainer/Dockerfile)
[[ "$timeweb_mise_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
timeweb_install_script=$(mktemp)
trap 'rm -f "$timeweb_install_script"' EXIT
curl -fsSL https://mise.run -o "$timeweb_install_script"
MISE_VERSION="$timeweb_mise_version" MISE_INSTALL_PATH="$HOME/.local/bin/mise" sh "$timeweb_install_script"
export PATH="$HOME/.local/bin:$PATH"
mise trust "$PWD/mise.toml"
mise install --locked bun node npm:vite npm:typescript
# --skip-tools also skips normal tool activation. Export only the binaries
# selected above, so tasks and the Vite child process do not rely on shims.
for frontend_command in bun node tsc vite; do
  frontend_binary=$(mise which "$frontend_command")
  export PATH="$(dirname "$frontend_binary"):$PATH"
done
# The selected tools are installed above. A regular `run` auto-installs every
# tool in mise.toml, including backend and infrastructure tools.
mise run --skip-tools frontend:install
mise run --skip-tools frontend:build
