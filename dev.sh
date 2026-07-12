#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

pids=()

cleanup() {
  echo
  echo "Stopping services..."
  for pid in "${pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  docker stop workhub-pg >/dev/null 2>&1 || true
  echo "Done."
}
trap cleanup EXIT INT TERM

echo "==> Starting postgres"
if docker ps -a --format '{{.Names}}' | grep -qx workhub-pg; then
  docker start workhub-pg >/dev/null
else
  (cd "$BACKEND_DIR" && docker compose up -d)
fi

echo "==> Waiting for postgres to be healthy"
until [ "$(docker inspect -f '{{.State.Health.Status}}' workhub-pg 2>/dev/null)" = "healthy" ]; do
  sleep 1
done

echo "==> Starting backend (go run .)"
(cd "$BACKEND_DIR" && go run .) &
pids+=($!)

echo "==> Starting frontend (bun run dev)"
(cd "$FRONTEND_DIR" && bun run dev) &
pids+=($!)

wait
