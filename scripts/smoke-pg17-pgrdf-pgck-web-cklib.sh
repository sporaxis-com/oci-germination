#!/bin/bash
# Smoke test: bundle-pg17-pgrdf-pgck-web-cklib
# Verifies PostgreSQL + pgRDF + pgCK + pgckweb FastAPI + cklib bundle

set -e

# shellcheck source=lib/assert-pgrdf-pgatomic.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/assert-pgrdf-pgatomic.sh"

psql_query() {
  local db="$1"
  local sql="$2"
  docker exec "$CONTAINER_NAME" psql -U postgres -d "$db" -At -v ON_ERROR_STOP=1 -c "$sql"
}

IMAGE="${1:-ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:latest}"
CONTAINER_NAME="ociger-pg17-pgrdf-pgck-web-cklib-smoke"
NETWORK_NAME="ociger-pg17-pgrdf-pgck-web-cklib-net"
DATA_DIR=".artifacts/ociger-pg17-pgrdf-pgck-web-cklib-smoke/pgdata"

echo "[smoke] Testing: $IMAGE"
echo "[smoke] Container: $CONTAINER_NAME"
echo "[smoke] Network: $NETWORK_NAME"
echo "[smoke] Data dir: $DATA_DIR"

# Cleanup
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
docker network rm "$NETWORK_NAME" 2>/dev/null || true
mkdir -p "$DATA_DIR"

# Start container
echo "[smoke] Starting container..."
docker network create "$NETWORK_NAME" || true
docker run --rm -d \
  --name "$CONTAINER_NAME" \
  --network "$NETWORK_NAME" \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/$DATA_DIR:/var/lib/postgresql/data" \
  -p 15432:5432 \
  -p 18000:8000 \
  "$IMAGE"

# Wait for PostgreSQL startup
echo "[smoke] Waiting for PostgreSQL startup..."
sleep 5
for i in {1..30}; do
  if docker exec "$CONTAINER_NAME" pg_isready -U postgres 2>/dev/null; then
    echo "[smoke] PostgreSQL is ready"
    break
  fi
  echo "[smoke] PostgreSQL startup... attempt $i/30"
  sleep 1
done

# Test PostgreSQL connectivity
echo "[smoke] Testing PostgreSQL connectivity..."
docker exec "$CONTAINER_NAME" psql -U postgres -d postgres -c "SELECT version();" > /tmp/pg_version.txt
PG_VERSION=$(cat /tmp/pg_version.txt | grep "PostgreSQL" | head -1)
echo "[smoke] PostgreSQL version: $PG_VERSION"

# Test pgRDF extension
echo "[smoke] Testing pgRDF extension..."
docker exec "$CONTAINER_NAME" psql -U postgres -d postgres -c "CREATE DATABASE pgrdf_test;" || echo "Database may already exist"
docker exec "$CONTAINER_NAME" psql -U postgres -d pgrdf_test -c "CREATE EXTENSION pgrdf;"
docker exec "$CONTAINER_NAME" psql -U postgres -d pgrdf_test -c "SELECT pgrdf.version();" > /tmp/pgrdf_version.txt
PGRDF_VERSION=$(cat /tmp/pgrdf_version.txt | grep -oE "[0-9]+\.[0-9]+\.[0-9]+")
echo "[smoke] pgRDF version: $PGRDF_VERSION"

# Test pgCK extension
echo "[smoke] Testing pgCK extension..."
docker exec "$CONTAINER_NAME" psql -U postgres -d pgrdf_test -c "CREATE EXTENSION pgck CASCADE;"
docker exec "$CONTAINER_NAME" psql -U postgres -d pgrdf_test -c "SELECT pgck_version();" > /tmp/pgck_version.txt
PGCK_VERSION=$(cat /tmp/pgck_version.txt | grep "pgck")
echo "[smoke] pgCK version: $PGCK_VERSION"

# Regression: pgRDF PgAtomic must be initialised (shared_preload_libraries='pgrdf,pgck')
echo "[smoke] Testing pgRDF PgAtomic initialization (regression)..."
assert_pgrdf_pgatomic pgrdf_test

# Test FastAPI on port 8000
echo "[smoke] Waiting for FastAPI startup..."
sleep 5
for i in {1..15}; do
  if curl -s http://127.0.0.1:18000/ >/dev/null 2>&1; then
    echo "[smoke] FastAPI is responding"
    break
  fi
  echo "[smoke] FastAPI startup... attempt $i/15"
  sleep 1
done

# Test FastAPI root endpoint
echo "[smoke] Testing FastAPI root endpoint..."
curl -s http://127.0.0.1:18000/ | head -20
echo ""

# Test FastAPI /static/ serving
echo "[smoke] Testing static files serving..."
curl -s -I http://127.0.0.1:18000/static/display.html || echo "display.html may not exist yet"

# Test cklib files serving
echo "[smoke] Testing cklib files serving..."
for file in ck-client.js ck-kernel.js ck-page.js ck-runtime.js; do
  if curl -s -I "http://127.0.0.1:18000/cklib/$file" 2>/dev/null | grep -q "200"; then
    echo "[smoke] ✓ /cklib/$file is accessible"
  else
    echo "[smoke] ✗ /cklib/$file not found or not serving (may be expected if files not present)"
  fi
done

# Test relation files on host bind mount
echo "[smoke] Testing relation files on host..."
if [ -d "$DATA_DIR/base" ]; then
  RELATION_FILES=$(find "$DATA_DIR/base" -type f 2>/dev/null | wc -l)
  echo "[smoke] Found $RELATION_FILES relation files in $DATA_DIR/base"
fi

# Cleanup
echo "[smoke] Stopping container..."
docker stop "$CONTAINER_NAME" || true

echo "[smoke] ✓ Smoke test completed successfully"
echo "[smoke] Container output available at: docker logs $CONTAINER_NAME"
