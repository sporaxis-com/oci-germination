#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/assert-pgrdf-pgatomic.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/assert-pgrdf-pgatomic.sh"
DATA_DIR="$ROOT/.artifacts/ociger-pg17-pgrdf-pgck-smoke/pgdata"
NETWORK="ociger-pg17-pgrdf-pgck-net"
CONTAINER="ociger-pg17-pgrdf-pgck-smoke"
OWNERSHIP_LABEL="pg17-pgrdf-pgck-smoke"
IMAGE="${1:-ociger-pg17-pgrdf-pgck:local}"
EXPECTED_PGRDF_VERSION="${PGRDF_EXPECTED_VERSION:-0.5.1}"
EXPECTED_PGCK_VERSION="${PGCK_EXPECTED_VERSION:-0.1.2}"
EXPECTED_PGCK_NATIVE_VERSION="${PGCK_EXPECTED_NATIVE_VERSION:-pgck 0.1.2 (rc3)}"

ensure_repo_data_path() {
  case "$DATA_DIR" in
    "$ROOT"/.artifacts/ociger-*) ;;
    *)
      echo "refusing to purge non-ociger data path: $DATA_DIR" >&2
      exit 1
      ;;
  esac
}

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

require_equal() {
  local label="$1"
  local actual="$2"
  local expected="$3"

  if [[ -z "$actual" ]]; then
    echo "native-version-empty: ${label} returned empty" >&2
    exit 1
  fi

  if [[ "$actual" != "$expected" ]]; then
    echo "native-version-mismatch: ${label}=${actual} expected=${expected}" >&2
    exit 1
  fi
}

cleanup() {
  remove_owned_container "$CONTAINER"
  remove_owned_network "$NETWORK"
}

trap cleanup EXIT

ensure_repo_data_path
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

for _ in $(seq 1 60); do
  if docker run --rm --network "$NETWORK" postgres:17-bookworm \
    pg_isready -h "$CONTAINER" -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

PGRDF_AVAILABLE="$(psql_query postgres "SELECT default_version FROM pg_available_extensions WHERE name='pgrdf';")"
if [[ -z "$PGRDF_AVAILABLE" ]]; then
  echo "artifact-missing: pg_available_extensions has no pgrdf row" >&2
  exit 1
fi

PGCK_AVAILABLE="$(psql_query postgres "SELECT default_version FROM pg_available_extensions WHERE name='pgck';")"
if [[ -z "$PGCK_AVAILABLE" ]]; then
  echo "artifact-missing: pg_available_extensions has no pgck row" >&2
  exit 1
fi

docker run --rm -i --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
CREATE EXTENSION pgrdf;
CREATE EXTENSION pgck CASCADE;
SQL

PGRDF_INSTALLED="$(psql_query postgres "SELECT extversion FROM pg_extension WHERE extname='pgrdf';")"
if [[ -z "$PGRDF_INSTALLED" ]]; then
  echo "not-installed: pg_extension has no extversion for pgrdf" >&2
  exit 1
fi

PGCK_INSTALLED="$(psql_query postgres "SELECT extversion FROM pg_extension WHERE extname='pgck';")"
if [[ -z "$PGCK_INSTALLED" ]]; then
  echo "not-installed: pg_extension has no extversion for pgck" >&2
  exit 1
fi

PGRDF_NATIVE="$(psql_query postgres "SELECT pgrdf.version();")"
PGCK_NATIVE="$(psql_query postgres "SELECT pgck_version();")"

assert_pgrdf_pgatomic postgres

if [[ "$PGRDF_AVAILABLE" != "$EXPECTED_PGRDF_VERSION" ]]; then
  echo "wrong-version: pgrdf available=$PGRDF_AVAILABLE expected=$EXPECTED_PGRDF_VERSION" >&2
  exit 1
fi

if [[ "$PGCK_AVAILABLE" != "$EXPECTED_PGCK_VERSION" ]]; then
  echo "wrong-version: pgck available=$PGCK_AVAILABLE expected=$EXPECTED_PGCK_VERSION" >&2
  exit 1
fi

if [[ "$PGRDF_INSTALLED" != "$EXPECTED_PGRDF_VERSION" ]]; then
  echo "wrong-version: pgrdf extversion=$PGRDF_INSTALLED expected=$EXPECTED_PGRDF_VERSION" >&2
  exit 1
fi

if [[ "$PGCK_INSTALLED" != "$EXPECTED_PGCK_VERSION" ]]; then
  echo "wrong-version: pgck extversion=$PGCK_INSTALLED expected=$EXPECTED_PGCK_VERSION" >&2
  exit 1
fi

require_equal "pgrdf.version()" "$PGRDF_NATIVE" "$EXPECTED_PGRDF_VERSION"
require_equal "pgck_version()" "$PGCK_NATIVE" "$EXPECTED_PGCK_NATIVE_VERSION"

echo "pgrdf.pg_available_extensions.default_version=$PGRDF_AVAILABLE"
echo "pgck.pg_available_extensions.default_version=$PGCK_AVAILABLE"
echo "pgrdf.pg_extension.extversion=$PGRDF_INSTALLED"
echo "pgck.pg_extension.extversion=$PGCK_INSTALLED"
echo "pgrdf.version()=$PGRDF_NATIVE"
echo "pgck_version()=$PGCK_NATIVE"
