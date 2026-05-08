#!/usr/bin/env bash
# scripts/dev.sh — foreground launcher for `make dev`.
# Runs quorant-api and quorant-worker under air with prefixed logs.
# Ctrl-C tears both down.
set -euo pipefail
set -m  # job control so each background pipeline is its own process group

# Auto-source .env so air-spawned binaries inherit env vars.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

if ! command -v air >/dev/null 2>&1; then
  echo "air not found on PATH. Install with:" >&2
  echo "  go install github.com/air-verse/air@latest" >&2
  echo "Then ensure \$(go env GOPATH)/bin is on your PATH." >&2
  exit 1
fi

cleanup() {
  # Three-layer cleanup. Air sometimes spawns the child binary in a way that
  # escapes the parent process group, so name-pattern pkill is needed too.
  local pids
  pids=$(jobs -p)
  # 1. SIGTERM each background job's entire process group.
  for pid in $pids; do
    kill -TERM "-$pid" 2>/dev/null || true
  done
  # 2. SIGTERM by name pattern, in case any binary escaped the group.
  pkill -TERM -f "/bin/quorant-(api|worker)$" 2>/dev/null || true
  pkill -TERM -f "air -c cmd/quorant-(api|worker)/.air.toml" 2>/dev/null || true
  # 3. Brief grace, then SIGKILL anything that didn't honor TERM.
  sleep 1
  for pid in $pids; do
    kill -KILL "-$pid" 2>/dev/null || true
  done
  pkill -KILL -f "/bin/quorant-(api|worker)$" 2>/dev/null || true
  pkill -KILL -f "air -c cmd/quorant-(api|worker)/.air.toml" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

(cd backend && air -c cmd/quorant-api/.air.toml    2>&1 | sed -u 's/^/[api]    /') &
(cd backend && air -c cmd/quorant-worker/.air.toml 2>&1 | sed -u 's/^/[worker] /') &
wait
