#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/assert-pgrdf-pgatomic.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/assert-pgrdf-pgatomic.sh"

DATA_DIR="$ROOT/.artifacts/ociger-pg17-pgrdf-pgck-static-cklib-smoke/pgdata"
NETWORK="ociger-pg17-pgrdf-pgck-static-cklib-net"
CONTAINER="ociger-pg17-pgrdf-pgck-static-cklib-smoke"
OWNERSHIP_LABEL="pg17-pgrdf-pgck-static-cklib-smoke"
IMAGE="${1:-ociger-pg17-pgrdf-pgck-static-cklib:local}"
EXPECTED_PGRDF_VERSION="${PGRDF_EXPECTED_VERSION:-0.5.28}"
EXPECTED_PGCK_VERSION="${PGCK_EXPECTED_VERSION:-0.2.2}"
EXPECTED_NATS_VERSION="${NATS_EXPECTED_VERSION:-2.14.1}"
EXPECTED_CKLIB_VERSION="${CKLIB_EXPECTED_VERSION:-1.3.0}"

owned_container_label() {
  docker inspect -f '{{ index .Config.Labels "io.sporaxis.ociger" }}' "$1" 2>/dev/null || true
}

owned_network_label() {
  docker network inspect -f '{{ index .Labels "io.sporaxis.ociger" }}' "$1" 2>/dev/null || true
}

remove_owned_container() {
  local name="$1"
  if docker inspect "$name" >/dev/null 2>&1; then
    local label
    label="$(owned_container_label "$name")"
    if [[ "$label" != "$OWNERSHIP_LABEL" ]]; then
      echo "refusing to remove container '$name' with label '$label'" >&2
      exit 1
    fi
    docker rm -f "$name" >/dev/null
  fi
}

remove_owned_network() {
  local name="$1"
  if docker network inspect "$name" >/dev/null 2>&1; then
    local label
    label="$(owned_network_label "$name")"
    if [[ "$label" != "$OWNERSHIP_LABEL" ]]; then
      echo "refusing to remove network '$name' with label '$label'" >&2
      exit 1
    fi
    docker network rm "$name" >/dev/null
  fi
}

psql_query() {
  local db="$1"
  local sql="$2"
  docker run --rm --network "$NETWORK" postgres:17-bookworm \
    psql -h "$CONTAINER" -U postgres -d "$db" -At -v ON_ERROR_STOP=1 -c "$sql"
}

cleanup() {
  remove_owned_container "$CONTAINER"
  remove_owned_network "$NETWORK"
}

trap cleanup EXIT

cleanup
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

docker network create \
  --label "io.sporaxis.ociger=$OWNERSHIP_LABEL" \
  "$NETWORK" >/dev/null

docker run -d \
  --name "$CONTAINER" \
  --label "io.sporaxis.ociger=$OWNERSHIP_LABEL" \
  --network "$NETWORK" \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$DATA_DIR:/var/lib/postgresql/data" \
  "$IMAGE" >/dev/null

# Wait for PostgreSQL
for _ in $(seq 1 60); do
  if docker run --rm --network "$NETWORK" postgres:17-bookworm \
    pg_isready -h "$CONTAINER" -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# Wait for NATS INFO line and assert version
INFO_LINE=""
for _ in $(seq 1 60); do
  INFO_LINE="$(
    docker run --rm --network "$NETWORK" busybox:1.37.0 \
      sh -c "nc -w 2 '$CONTAINER' 4222 < /dev/null | head -n 1"
  )"
  case "$INFO_LINE" in
    INFO\ *) break ;;
  esac
  sleep 1
done
case "$INFO_LINE" in
  INFO\ *\"version\":\"$EXPECTED_NATS_VERSION\"*) ;;
  *) echo "wrong-version: nats INFO line missing $EXPECTED_NATS_VERSION: $INFO_LINE" >&2; exit 1 ;;
esac

# Wait for NATS WSS port 9222
for _ in $(seq 1 60); do
  if docker run --rm --network "$NETWORK" busybox:1.37.0 \
    sh -c "nc -zvw 2 '$CONTAINER' 9222" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker run --rm --network "$NETWORK" busybox:1.37.0 \
  sh -c "nc -zvw 2 '$CONTAINER' 9222" >/dev/null

# Wait for static server on 8000 (the v3.8-distinguishing service)
HEALTHZ=""
for _ in $(seq 1 60); do
  HEALTHZ="$(
    docker run --rm --network "$NETWORK" curlimages/curl:8.10.1 \
      -sf "http://$CONTAINER:8000/healthz" 2>/dev/null || true
  )"
  if [[ "$HEALTHZ" == "ok" ]]; then
    break
  fi
  sleep 1
done
if [[ "$HEALTHZ" != "ok" ]]; then
  echo "static-server /healthz did not return 'ok'; got: '$HEALTHZ'" >&2
  exit 1
fi

# Verify cklib is served and contains expected content
CKLIB_BODY="$(
  docker run --rm --network "$NETWORK" curlimages/curl:8.10.1 \
    -sf "http://$CONTAINER:8000/cklib/ck-client.js"
)"
if [[ -z "$CKLIB_BODY" ]]; then
  echo "cklib-missing: /cklib/ck-client.js returned empty body" >&2
  exit 1
fi
case "$CKLIB_BODY" in
  *"CK Web Client"*) ;;
  *)
    echo "cklib-content-unexpected: /cklib/ck-client.js missing expected CK Web Client marker" >&2
    echo "first 200 chars: ${CKLIB_BODY:0:200}" >&2
    exit 1
    ;;
esac

# Verify NO Python/FastAPI in the image (the distinguishing v3.8 property)
HAS_PYTHON="$(docker run --rm --entrypoint /bin/sh "$IMAGE" -c "command -v python || command -v python3 || echo none" 2>&1 || echo none)"
case "$HAS_PYTHON" in
  none|*"executable file not found"*) ;;
  *)
    echo "unexpected: static-only bundle contains Python interpreter at $HAS_PYTHON" >&2
    exit 1
    ;;
esac

# Verify extensions install + version + PgAtomic regression
docker run --rm -i --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
CREATE DATABASE ociger_demo;
SQL
docker run --rm -i --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d ociger_demo -v ON_ERROR_STOP=1 <<'SQL'
CREATE EXTENSION pgrdf;
CREATE EXTENSION pgck CASCADE;
SQL

PGRDF_INSTALLED="$(psql_query ociger_demo "SELECT extversion FROM pg_extension WHERE extname='pgrdf';")"
PGCK_INSTALLED="$(psql_query ociger_demo "SELECT extversion FROM pg_extension WHERE extname='pgck';")"

if [[ "$PGRDF_INSTALLED" != "$EXPECTED_PGRDF_VERSION" ]]; then
  echo "wrong-version: pgrdf extversion=$PGRDF_INSTALLED expected=$EXPECTED_PGRDF_VERSION" >&2
  exit 1
fi
if [[ "$PGCK_INSTALLED" != "$EXPECTED_PGCK_VERSION" ]]; then
  echo "wrong-version: pgck extversion=$PGCK_INSTALLED expected=$EXPECTED_PGCK_VERSION" >&2
  exit 1
fi

# Critical: PgAtomic init must work (pgRDF parse_turtle)
assert_pgrdf_pgatomic ociger_demo

echo "nats_info_line=$INFO_LINE"
echo "static.healthz=$HEALTHZ"
echo "static.cklib_first_60=${CKLIB_BODY:0:60}"
echo "static.has_python=$HAS_PYTHON"
echo "pgrdf.pg_extension.extversion=$PGRDF_INSTALLED"
echo "pgck.pg_extension.extversion=$PGCK_INSTALLED"
echo "cklib.expected_version=$EXPECTED_CKLIB_VERSION"
echo "smoke=pass"
