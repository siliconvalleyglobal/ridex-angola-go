#!/usr/bin/env bash
# Start Postgres, Redis, migrate, seed, then API + matching + worker.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export APP_ENV="${APP_ENV:-development}"
export SERVER_PORT="${SERVER_PORT:-8080}"
export DB_HOST="${DB_HOST:-127.0.0.1}"
export DB_PORT="${DB_PORT:-5432}"
export DB_USER="${DB_USER:-ridex}"
export DB_PASSWORD="${DB_PASSWORD:-ridex_secret}"
export DB_NAME="${DB_NAME:-ridex}"
export DB_SSL_MODE="${DB_SSL_MODE:-disable}"
export SMS_PROVIDER="${SMS_PROVIDER:-none}"
export PAYMENT_PROVIDER="${PAYMENT_PROVIDER:-none}"
export LOG_LEVEL="${LOG_LEVEL:-debug}"
export JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-dev-access-change-me}"
export JWT_REFRESH_SECRET="${JWT_REFRESH_SECRET:-dev-refresh-change-me}"

if command -v pg_ctlcluster >/dev/null 2>&1; then
  sudo pg_ctlcluster 16 main start >/dev/null 2>&1 || true
fi
if command -v redis-server >/dev/null 2>&1; then
  redis-cli ping >/dev/null 2>&1 || redis-server --daemonize yes --port 6379 --save "" --appendonly no
fi

go run ./cmd/migrate -command up
go run ./cmd/seed

mkdir -p .tmp
go run ./cmd/api >.tmp/api.log 2>&1 &
echo $! >.tmp/api.pid
go run ./cmd/matching >.tmp/matching.log 2>&1 &
echo $! >.tmp/matching.pid
go run ./cmd/worker >.tmp/worker.log 2>&1 &
echo $! >.tmp/worker.pid

for i in $(seq 1 40); do
  if curl -sf "http://127.0.0.1:${SERVER_PORT}/readyz" >/dev/null; then
    echo "API ready at http://127.0.0.1:${SERVER_PORT}/app"
    echo "Mobile alias: http://127.0.0.1:${SERVER_PORT}/mobile"
    exit 0
  fi
  sleep 0.5
done
echo "API did not become ready; last log:" >&2
tail -40 .tmp/api.log >&2
exit 1
