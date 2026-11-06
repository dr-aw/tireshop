#!/usr/bin/env bash
# Compare with the last successful CI, so failed/cancelled pushes are not lost.
set -euo pipefail
backend=false frontend=false infra=false markdown=false
full_check() {
  backend=true frontend=true infra=true markdown=true
}
base=${1:-}
head=${2:-HEAD}
changes=$(mktemp)
trap 'rm -f "$changes"' EXIT
if [[ -z "$base" ]] || ! git merge-base --is-ancestor "$base" "$head" 2>/dev/null \
  || ! git diff --name-only --no-renames -z "$base" "$head" -- > "$changes"; then
  # First/manual run, missing history or non-ancestor: never guess what to skip.
  full_check
else
  while IFS= read -r -d '' path; do
    case "$path" in
      .github/*|.devcontainer/*|mise*.toml|mise*.lock|compose*.yaml|compose*.yml|.dockerignore|.env.example|.gitignore|deploy/api.Dockerfile)
        full_check ;;
      docs/contracts/*)
        backend=true frontend=true markdown=true ;;
      *.md|.rumdl.toml)
        markdown=true ;;
      frontend/*|scripts/render-static-build.sh|scripts/catalog-*.mjs)
        frontend=true ;;
      cmd/*|internal/*|go.mod|go.sum|go.work|go.work.sum|migrations/*|deploy/aiven/*|scripts/*)
        backend=true frontend=true ;;
      deploy/terraform/*)
        infra=true ;;
      .pre-commit-config.yaml|.vscode/*)
        # Repository checks below still run; these do not affect runtime builds.
        ;;
      *)
        # A new directory/file must be explicitly classified before skipping it.
        full_check ;;
    esac
  done < "$changes"
fi
printf 'backend=%s\nfrontend=%s\ninfra=%s\nmarkdown=%s\n' \
  "$backend" "$frontend" "$infra" "$markdown"
