#!/bin/bash
# SoloQuest Dev Database Reset Script
#
# SAFETY: Only runs when ALLOW_DEV_DB_RESET=true AND APP_ENV is development.
# Never usable in production or against non-dev databases.
#
# What this does:
#   1. Verifies safety preconditions (APP_ENV, DB name, env flag)
#   2. Drops the public schema (CASCADE)
#   3. Recreates the public schema
#   4. Recreates required extensions (pgcrypto)
#   5. Clears golang-migrate tracking table (schema_migrations)
#   6. Next app start will re-run all migrations from scratch and re-seed dev data
#
# Usage:
#   ALLOW_DEV_DB_RESET=true make db-reset-dev
#   ALLOW_DEV_DB_RESET=true ./scripts/reset_dev_db.sh

set -euo pipefail

# ─── 1. Safety preconditions ─────────────────────────────────────────────────

if [ "${ALLOW_DEV_DB_RESET:-}" != "true" ]; then
  echo "ERROR: ALLOW_DEV_DB_RESET must be set to 'true' to reset the dev database."
  echo ""
  echo "Usage:"
  echo "  ALLOW_DEV_DB_RESET=true make db-reset-dev"
  echo "  ALLOW_DEV_DB_RESET=true ./scripts/reset_dev_db.sh"
  exit 1
fi

if [ "${APP_ENV:-}" != "development" ]; then
  echo "ERROR: APP_ENV must be 'development' (current: '${APP_ENV:-not set}')."
  echo "This script is only for local development. Aborting."
  exit 1
fi

# ─── 2. Determine database connection ────────────────────────────────────────

CONTAINER_NAME="${DB_CONTAINER_NAME:-soloquest-db}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-soloquest}"

echo "──────────────────────────────────────────────────"
echo "  SoloQuest Dev Database Reset"
echo "──────────────────────────────────────────────────"
echo "  Container : ${CONTAINER_NAME}"
echo "  User      : ${DB_USER}"
echo "  Database  : ${DB_NAME}"
echo "  APP_ENV   : ${APP_ENV}"
echo "──────────────────────────────────────────────────"

# ─── 3. Verify database name looks like a dev/local database ─────────────────

if [[ "${DB_NAME}" != "soloquest" ]] && [[ "${DB_NAME}" != *"dev"* ]] && [[ "${DB_NAME}" != *"test"* ]] && [[ "${DB_NAME}" != *"local"* ]]; then
  echo "ERROR: DB_NAME '${DB_NAME}' does not look like a dev/test database."
  echo "Expected one of: soloquest, or name containing 'dev', 'test', or 'local'."
  echo "Aborting to prevent accidental production reset."
  exit 1
fi

# ─── 4. Confirm before proceeding ────────────────────────────────────────────

echo ""
echo "WARNING: This will DROP and RECREATE the '${DB_NAME}' database schema."
echo "All data will be permanently lost."
echo ""
read -rp "Type 'yes' to confirm: " CONFIRM
if [ "${CONFIRM}" != "yes" ]; then
  echo "Aborted."
  exit 0
fi
echo ""

# ─── 5. Drop and recreate the public schema ──────────────────────────────────

echo "[1/3] Dropping public schema (CASCADE)..."

docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "DROP SCHEMA IF EXISTS public CASCADE;" 2>&1

echo "       Schema dropped."

echo "[2/3] Recreating public schema..."

docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "CREATE SCHEMA public;" 2>&1

echo "       Schema recreated."

# ─── 6. Recreate required extensions ─────────────────────────────────────────

echo "[3/3] Recreating extensions..."

docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "CREATE EXTENSION IF NOT EXISTS \"pgcrypto\";" 2>&1

echo "       Extensions ready."

# ─── 7. Done ─────────────────────────────────────────────────────────────────

echo ""
echo "──────────────────────────────────────────────────"
echo "  Dev database reset complete."
echo ""
echo "  The public schema has been dropped and recreated."
echo "  Next app start will:"
echo "    - Run all SQL migrations 001 through 014"
echo "    - Seed default dev user and data"
echo "──────────────────────────────────────────────────"
echo ""
echo "You can now start the backend with:"
echo "  make run"
echo "  or: go run ./cmd/api/main.go"
echo ""
