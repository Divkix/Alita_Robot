#!/usr/bin/env bash
# Per-boot startup for the Alita Robot Cloud Agent environment.
# Starts PostgreSQL and Redis, ensures the application database exists, applies
# SQL migrations (idempotent), and writes a local .env with local service
# defaults. Secrets (BOT_TOKEN, OWNER_ID, MESSAGE_DUMP) are injected by Cursor
# as environment variables and are NOT written here.
set -euo pipefail

DB_NAME="alita_robot"
DB_USER="postgres"
DB_PASSWORD="password"
DB_PORT="5432"

echo "==> [start] Starting PostgreSQL"
PG_VER="$(ls /etc/postgresql 2>/dev/null | sort -V | tail -1 || true)"
if [ -z "${PG_VER}" ]; then
  echo "!! PostgreSQL is not installed; run .cursor/install.sh first" >&2
  exit 1
fi
if ! sudo pg_ctlcluster "${PG_VER}" main status >/dev/null 2>&1; then
  sudo pg_ctlcluster "${PG_VER}" main start
fi
# Wait for the server to accept connections.
for _ in $(seq 1 30); do
  if pg_isready -h localhost -p "${DB_PORT}" >/dev/null 2>&1; then break; fi
  sleep 1
done

echo "==> [start] Ensuring database role and database exist"
sudo -u postgres psql -v ON_ERROR_STOP=1 -c "ALTER USER ${DB_USER} PASSWORD '${DB_PASSWORD}';"
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
  sudo -u postgres createdb "${DB_NAME}"
fi

echo "==> [start] Starting Redis"
if ! redis-cli ping >/dev/null 2>&1; then
  sudo redis-server /etc/redis/redis.conf --daemonize yes
  for _ in $(seq 1 15); do
    if redis-cli ping >/dev/null 2>&1; then break; fi
    sleep 1
  done
fi

echo "==> [start] Applying database migrations (idempotent)"
PSQL_DB_HOST=localhost \
PSQL_DB_PORT="${DB_PORT}" \
PSQL_DB_NAME="${DB_NAME}" \
PSQL_DB_USER="${DB_USER}" \
PSQL_DB_PASSWORD="${DB_PASSWORD}" \
PSQL_DB_SSLMODE=disable \
  bash scripts/migrate_psql.sh

echo "==> [start] Writing local .env with service defaults (if missing)"
if [ ! -f .env ]; then
  cat > .env <<EOF
# Local development defaults written by .cursor/start.sh.
# Real secrets (BOT_TOKEN, OWNER_ID, MESSAGE_DUMP) come from Cursor-injected env
# vars, which take precedence over these values (godotenv does not override).
DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable
REDIS_ADDRESS=localhost:6379
REDIS_DB=1
HTTP_PORT=8080
AUTO_MIGRATE=false
DEBUG=true
DROP_PENDING_UPDATES=true
ENABLED_LOCALES=en
EOF
fi

echo "==> [start] Ready. Postgres + Redis are up and migrations are applied."
echo "    Run the bot with: make run   (requires BOT_TOKEN/OWNER_ID/MESSAGE_DUMP secrets)"
