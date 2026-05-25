#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="$ROOT/.artifacts/ociger-core-pg17-smoke/pgdata"
NETWORK="ociger-core-pg17-net"
CONTAINER="ociger-core-pg17-smoke"
OWNERSHIP_LABEL="core-pg17-smoke"
IMAGE="${1:-ociger-core-pg17-min:local}"
RELATIVE_PATH=""

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

docker run --rm -i --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
CREATE DATABASE ociger_demo;
SQL

docker run --rm -i --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d ociger_demo -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE public.demo_rows (
  id integer primary key,
  note text not null
);
INSERT INTO public.demo_rows (id, note) VALUES (1, 'ociger smoke row');
SELECT id, note FROM public.demo_rows ORDER BY id;
SQL

RELATIVE_PATH="$(
  docker run --rm --network "$NETWORK" postgres:17-bookworm \
    psql -h "$CONTAINER" -U postgres -d ociger_demo -At -v ON_ERROR_STOP=1 \
      -c "SELECT pg_relation_filepath('public.demo_rows'::regclass);"
)"

HOST_PATH="$DATA_DIR/$RELATIVE_PATH"

if [[ ! -f "$HOST_PATH" ]]; then
  echo "expected relation file not found: $HOST_PATH" >&2
  exit 1
fi

echo "pg_relation_filepath(public.demo_rows)=$RELATIVE_PATH"
echo "host_path=$HOST_PATH"
