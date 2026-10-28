#!/usr/bin/env bash
# Run all three independently built programs for local development. Each API
# still has its own entry point, port, restricted DB login and lifecycle.
set -euo pipefail
cd "$(dirname "$0")/.."
# This task serves local HTTP only. Independently started APIs default to Secure.
export ADMIN_LOCAL_HTTP=1
mkdir -p build
mise exec -- go build -o build/catalog ./cmd/catalog
mise exec -- go build -o build/requests ./cmd/requests
pids=()
cleanup() {
  trap - EXIT
  if ((${#pids[@]})); then
    kill "${pids[@]}" 2>/dev/null || true
    wait "${pids[@]}" 2>/dev/null || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
./build/catalog &
pids+=("$!")
./build/requests &
pids+=("$!")
# Fail before opening the UI if either service cannot start with its own login.
ready=false
for attempt in {1..50}; do
  kill -0 "${pids[0]}" "${pids[1]}" 2>/dev/null || exit 1
  if curl -fsS http://127.0.0.1:8081/readyz >/dev/null 2>&1 && curl -fsS http://127.0.0.1:8082/readyz >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.2
done
if [[ "$ready" != true ]]; then echo 'API readiness timed out.' >&2; exit 1; fi
mise exec -- vite frontend --host 0.0.0.0 --port 5173 --strictPort &
pids+=("$!")
wait -n "${pids[@]}"
