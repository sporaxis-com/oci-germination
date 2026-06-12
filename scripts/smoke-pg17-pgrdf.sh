#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/assert-pgrdf-pgatomic.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/assert-pgrdf-pgatomic.sh"
DATA_DIR="$ROOT/.artifacts/ociger-pg17-pgrdf-smoke/pgdata"
NETWORK="ociger-pg17-pgrdf-net"
CONTAINER="ociger-pg17-pgrdf-smoke"
OWNERSHIP_LABEL="pg17-pgrdf-smoke"
IMAGE="${1:-ociger-pg17-pgrdf:local}"
EXPECTED_VERSION="${PGRDF_EXPECTED_VERSION:-0.6.0}"

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

AVAILABLE_DEFAULT="$(psql_query postgres "SELECT default_version FROM pg_available_extensions WHERE name='pgrdf';")"
if [[ -z "$AVAILABLE_DEFAULT" ]]; then
  echo "artifact-missing: pg_available_extensions has no pgrdf row" >&2
  exit 1
fi

docker run --rm -i --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
CREATE EXTENSION pgrdf;
SQL

INSTALLED_VERSION="$(psql_query postgres "SELECT extversion FROM pg_extension WHERE extname='pgrdf';")"
if [[ -z "$INSTALLED_VERSION" ]]; then
  echo "not-installed: pg_extension has no extversion for pgrdf" >&2
  exit 1
fi

NATIVE_VERSION="$(psql_query postgres "SELECT pgrdf.version();")"
if [[ -z "$NATIVE_VERSION" ]]; then
  echo "native-version-empty: pgrdf.version() returned empty" >&2
  exit 1
fi

assert_pgrdf_pgatomic postgres

if [[ "$INSTALLED_VERSION" != "$EXPECTED_VERSION" ]]; then
  echo "wrong-version: extversion=$INSTALLED_VERSION expected=$EXPECTED_VERSION" >&2
  exit 1
fi

if [[ "$NATIVE_VERSION" != "$EXPECTED_VERSION" ]]; then
  echo "native-version-mismatch: pgrdf.version()=$NATIVE_VERSION expected=$EXPECTED_VERSION" >&2
  exit 1
fi

echo "pg_available_extensions.default_version=$AVAILABLE_DEFAULT"
echo "pg_extension.extversion=$INSTALLED_VERSION"
echo "pgrdf.version()=$NATIVE_VERSION"
