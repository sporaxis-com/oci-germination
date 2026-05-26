#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="$ROOT/.artifacts/ociger-core-pg17-nats-smoke/pgdata"
NETWORK="ociger-core-pg17-nats-net"
CONTAINER="ociger-core-pg17-nats-smoke"
OWNERSHIP_LABEL="core-pg17-nats-smoke"
IMAGE="${1:-ociger-core-pg17-nats:local}"
RELATIVE_PATH=""
RELATION_PROOF_METHOD=""
INFO_LINE=""

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
  INFO\ *) ;;
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
echo "pg_relation_filepath(public.demo_rows)=$RELATIVE_PATH"
echo "host_path=$HOST_PATH"
echo "relation_proof_method=$RELATION_PROOF_METHOD"
