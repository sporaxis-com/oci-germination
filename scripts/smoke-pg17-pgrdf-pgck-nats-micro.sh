#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT_PHYSICAL="$(cd -P "$ROOT" && pwd)"
# shellcheck source=lib/assert-pgrdf-pgatomic.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/assert-pgrdf-pgatomic.sh"
DATA_DIR="$ROOT/.artifacts/ociger-pg17-pgrdf-pgck-nats-micro-smoke/pgdata"
NETWORK="ociger-pg17-pgrdf-pgck-nats-micro-net"
CONTAINER="ociger-pg17-pgrdf-pgck-nats-micro-smoke"
OWNERSHIP_LABEL="pg17-pgrdf-pgck-nats-micro-smoke"
IMAGE="${1:-ociger-pg17-pgrdf-pgck-nats-micro:local}"
# Versions come from versions.yaml (single source of truth) via lib/versions.sh;
# env still overrides. pgck_native is carried there because pgCK 0.4.13 reports a
# stale "0.4.3 (rc3)" natively (extension is correctly 0.4.13 — pgCK NOTIFY filed).
source "$(dirname "${BASH_SOURCE[0]}")/lib/versions.sh"
EXPECTED_PGRDF_VERSION="${PGRDF_EXPECTED_VERSION:-$OCIGER_PGRDF_VERSION}"
EXPECTED_PGCK_VERSION="${PGCK_EXPECTED_VERSION:-$OCIGER_PGCK_VERSION}"
EXPECTED_PGCK_NATIVE_VERSION="${PGCK_EXPECTED_NATIVE_VERSION:-$OCIGER_PGCK_NATIVE}"
EXPECTED_NATS_VERSION="${NATS_EXPECTED_VERSION:-2.14.1}"
RELATIVE_PATH=""
RELATION_PROOF_METHOD=""
INFO_LINE=""

resolve_physical_dir() {
  (
    cd -P "$1" >/dev/null 2>&1 &&
      pwd
  )
}

ensure_repo_data_path() {
  local artifacts_dir data_parent data_parent_name data_basename
  local resolved_artifacts resolved_parent resolved_target

  case "$DATA_DIR" in
    "$ROOT"/.artifacts/ociger-*) ;;
    *)
      echo "refusing to purge non-ociger data path: $DATA_DIR" >&2
      exit 1
      ;;
  esac

  artifacts_dir="$ROOT/.artifacts"
  data_parent="${DATA_DIR%/*}"
  data_parent_name="${data_parent##*/}"
  data_basename="${DATA_DIR##*/}"

  if [[ -e "$artifacts_dir" || -L "$artifacts_dir" ]]; then
    resolved_artifacts="$(resolve_physical_dir "$artifacts_dir")" || {
      echo "refusing to purge unresolved artifacts path: $artifacts_dir" >&2
      exit 1
    }
  else
    resolved_artifacts="$ROOT_PHYSICAL/.artifacts"
  fi

  if [[ "$resolved_artifacts" != "$ROOT_PHYSICAL/.artifacts" ]]; then
    echo "refusing to purge symlinked artifacts path: $artifacts_dir -> $resolved_artifacts" >&2
    exit 1
  fi

  if [[ -e "$data_parent" || -L "$data_parent" ]]; then
    resolved_parent="$(resolve_physical_dir "$data_parent")" || {
      echo "refusing to purge unresolved data path: $data_parent" >&2
      exit 1
    }
  else
    resolved_parent="$resolved_artifacts/$data_parent_name"
  fi

  resolved_target="$resolved_parent/$data_basename"

  case "$resolved_target" in
    "$ROOT_PHYSICAL"/.artifacts/ociger-*)
      DATA_DIR="$resolved_target"
      ;;
    *)
      echo "refusing to purge symlinked data path: $DATA_DIR -> $resolved_target" >&2
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

relation_exists_via_helper() {
  docker run --rm \
    --entrypoint /bin/sh \
    -v "$DATA_DIR:/mnt/pgdata:ro" \
    postgres:17-bookworm \
    -c "test -f '/mnt/pgdata/$RELATIVE_PATH'"
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
  INFO\ *)
    echo "wrong-version: nats info line does not contain expected version $EXPECTED_NATS_VERSION: $INFO_LINE" >&2
    exit 1
    ;;
  *)
    echo "expected NATS INFO line on 4222, got: $INFO_LINE" >&2
    exit 1
    ;;
esac

for _ in $(seq 1 60); do
  if docker run --rm --network "$NETWORK" busybox:1.37.0 \
    sh -c "nc -zvw 2 '$CONTAINER' 9222" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker run --rm --network "$NETWORK" busybox:1.37.0 \
  sh -c "nc -zvw 2 '$CONTAINER' 9222"

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
CREATE DATABASE ociger_demo;
SQL

docker run --rm -i --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d ociger_demo -v ON_ERROR_STOP=1 <<'SQL'
CREATE EXTENSION pgrdf;
CREATE EXTENSION pgck CASCADE;
CREATE TABLE public.demo_rows (
  id integer primary key,
  note text not null
);
INSERT INTO public.demo_rows (id, note) VALUES (1, 'ociger smoke row');
SELECT id, note FROM public.demo_rows ORDER BY id;
SQL

PGRDF_INSTALLED="$(psql_query ociger_demo "SELECT extversion FROM pg_extension WHERE extname='pgrdf';")"
if [[ -z "$PGRDF_INSTALLED" ]]; then
  echo "not-installed: pg_extension has no extversion for pgrdf" >&2
  exit 1
fi

PGCK_INSTALLED="$(psql_query ociger_demo "SELECT extversion FROM pg_extension WHERE extname='pgck';")"
if [[ -z "$PGCK_INSTALLED" ]]; then
  echo "not-installed: pg_extension has no extversion for pgck" >&2
  exit 1
fi

PGRDF_NATIVE="$(psql_query ociger_demo "SELECT pgrdf.version();")"
PGCK_NATIVE="$(psql_query ociger_demo "SELECT pgck_version();")"

assert_pgrdf_pgatomic ociger_demo

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

RELATIVE_PATH="$(psql_query ociger_demo "SELECT pg_relation_filepath('public.demo_rows'::regclass);")"
HOST_PATH="$DATA_DIR/$RELATIVE_PATH"

for _ in $(seq 1 10); do
  if [[ -f "$HOST_PATH" ]]; then
    RELATION_PROOF_METHOD="host"
    break
  fi
  if relation_exists_via_helper; then
    RELATION_PROOF_METHOD="helper-container"
    break
  fi
  sleep 1
done

if [[ -z "$RELATION_PROOF_METHOD" ]]; then
  echo "expected relation file not found: $HOST_PATH" >&2
  docker run --rm \
    --entrypoint /bin/sh \
    -v "$DATA_DIR:/mnt/pgdata:ro" \
    postgres:17-bookworm \
    -c "find /mnt/pgdata/base -maxdepth 2 -type f | sort | tail -n 50" >&2 || true
  exit 1
fi

echo "nats_info_line=$INFO_LINE"
echo "pgrdf.pg_available_extensions.default_version=$PGRDF_AVAILABLE"
echo "pgck.pg_available_extensions.default_version=$PGCK_AVAILABLE"
echo "pgrdf.pg_extension.extversion=$PGRDF_INSTALLED"
echo "pgck.pg_extension.extversion=$PGCK_INSTALLED"
echo "pgrdf.version()=$PGRDF_NATIVE"
echo "pgck_version()=$PGCK_NATIVE"
echo "pg_relation_filepath(public.demo_rows)=$RELATIVE_PATH"
echo "host_path=$HOST_PATH"
echo "relation_proof_method=$RELATION_PROOF_METHOD"
