#!/usr/bin/env bash
# Disposable synthetic repository: no network, runtime services or secrets.
set -euo pipefail
classifier=$(cd "$(dirname "$0")" && pwd)/changes.sh
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
cd "$fixture"
git init -q
git config user.email ci@example.invalid
git config user.name 'CI filter test'
git commit -qm initial --allow-empty
initial=$(git rev-parse HEAD)
count=0
check() {
  local label=$1 base=$2 expected=$3 actual
  actual=$(bash "$classifier" "$base" HEAD)
  if [[ "$actual" != "$expected" ]]; then
    printf 'FAIL %s\nExpected:\n%s\nActual:\n%s\n' "$label" "$expected" "$actual" >&2
    exit 1
  fi
  count=$((count + 1))
}
none=$'backend=false\nfrontend=false\ninfra=false\nmarkdown=false'
front=$'backend=false\nfrontend=true\ninfra=false\nmarkdown=false'
app=$'backend=true\nfrontend=true\ninfra=false\nmarkdown=false'
all=$'backend=true\nfrontend=true\ninfra=true\nmarkdown=true'
check unchanged "$initial" "$none"
check manual '' "$all"
check missing-base 0000000000000000000000000000000000000000 "$all"
paths=(frontend/src/style.css frontend/bun.lock frontend/e2e/test.cjs
  scripts/render-static-build.sh scripts/catalog-media.mjs internal/httpapi/api.go
  go.mod migrations/catalog/001.sql deploy/aiven/bootstrap.sql scripts/test-db.sh
  deploy/api.Dockerfile .devcontainer/Dockerfile compose.yaml compose.agent.yaml
  .dockerignore mise.lock mise.agent.toml .github/workflows/ci.yml unknown/new.txt)
expected=("$front" "$front" "$front" "$front" "$front" "$app" "$app" "$app"
  "$app" "$app" "$all" "$all" "$all" "$all" "$all" "$all" "$all" "$all" "$all")
for i in "${!paths[@]}"; do
  path=${paths[$i]}
  mkdir -p "$(dirname "$path")"
  printf 'synthetic\n' > "$path"
  git add -- "$path"
  git commit -qm fixture
  check "$path" HEAD^ "${expected[$i]}"
done
mkdir -p docs deploy/terraform
printf 'doc\n' > docs/guide.md
git add .
git commit -qm docs
check docs HEAD^ $'backend=false\nfrontend=false\ninfra=false\nmarkdown=true'
printf 'infra\n' > deploy/terraform/main.tf
git add .
git commit -qm infra
check infra HEAD^ $'backend=false\nfrontend=false\ninfra=true\nmarkdown=false'
# Changes from multiple pushes since the last success must all be covered.
check multiple-commits HEAD~2 $'backend=false\nfrontend=false\ninfra=true\nmarkdown=true'
git mv frontend/src/style.css docs/removed-css.md
git commit -qm rename
check rename HEAD^ $'backend=false\nfrontend=true\ninfra=false\nmarkdown=true'
git rm -q frontend/bun.lock
git commit -qm delete
check deletion HEAD^ "$front"
printf 'newline filename\n' > $'frontend/src/a\nb.css'
git add .
git commit -qm newline
check newline HEAD^ "$front"
git checkout -q --orphan unrelated
git commit -qm unrelated --allow-empty
check non-ancestor "$initial" "$all"
printf 'PASS: %s CI change-filter scenarios\n' "$count"
