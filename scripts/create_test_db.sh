#!/bin/bash
set -e

CONTAINER_NAME="soloquest_postgres"
DB_NAME="soloquest_test"
DB_USER="postgres"

echo "Creating test database '$DB_NAME' if it does not exist..."

docker exec $CONTAINER_NAME psql -U $DB_USER -tc "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1 || \
  docker exec $CONTAINER_NAME psql -U $DB_USER -c "CREATE DATABASE $DB_NAME"

echo "Test database '$DB_NAME' is ready."
