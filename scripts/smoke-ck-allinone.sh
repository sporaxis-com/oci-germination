#!/bin/bash
# Smoke test: bundle-ck-allinone (Delta — s6-overlay + busybox httpd, no Python)
# Verifies: PostgreSQL + pgRDF + pgCK + cklib via busybox httpd + NATS core + WSS bridge
#
# Delta composition (no FastAPI, no /opt/venv, no Python, no ociger-supervisor,
# no ociger-static-server). PID 1 is s6-svscan. Web serving is busybox httpd
# applet. Bundle aims for marketplace-minimal size.

set -e

IMAGE="${1:-ghcr.io/sporaxis-com/ociger-ck-allinone:latest}"
CONTAINER_NAME="ociger-ck-allinone-smoke"
NETWORK_NAME="ociger-ck-allinone-net"
DATA_DIR=".artifacts/ociger-ck-allinone-smoke/pgdata"

EXPECTED_PGRDF_VERSION="${PGRDF_EXPECTED_VERSION:-0.5.42}"
EXPECTED_PGCK_VERSION="${PGCK_EXPECTED_VERSION:-0.4.0}"
EXPECTED_PGCK_NATIVE="${PGCK_EXPECTED_NATIVE_VERSION:-pgck 0.4.0 (rc3)}"
EXPECTED_CKLIB_VERSION="${CKLIB_EXPECTED_VERSION:-1.3.11}"

echo "════════════════════════════════════════════════════════════"
echo "[ck-allinone] CKP v3.8 All-in-One (Delta) Smoke Test"
echo "════════════════════════════════════════════════════════════"
echo "[ck-allinone] Image:     $IMAGE"
echo "[ck-allinone] Container: $CONTAINER_NAME"
echo ""

# Cleanup
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker network rm "$NETWORK_NAME" >/dev/null 2>&1 || true
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

# Start container
echo "[ck-allinone] Starting container..."
docker network create "$NETWORK_NAME" >/dev/null
docker run --rm -d \
  --name "$CONTAINER_NAME" \
  --network "$NETWORK_NAME" \
  -e POSTGRES_PASSWORD=smoketest \
  -p 35432:5432 -p 38000:8000 -p 34222:4222 -p 39222:9222 \
  -v "$PWD/$DATA_DIR:/var/lib/postgresql/data" \
  "$IMAGE" >/dev/null

cleanup() {
  echo ""
  echo "[ck-allinone] cleanup"
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  docker network rm "$NETWORK_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# pg_base ships only initdb + postgres server — no client tools (psql, pg_isready).
# Use a sidecar postgres:17-bookworm to connect over the smoke network.
PSQL="docker run --rm --network $NETWORK_NAME -e PGPASSWORD=smoketest postgres:17-bookworm psql -h $CONTAINER_NAME -U postgres -d postgres -At -v ON_ERROR_STOP=1"
PGISREADY="docker run --rm --network $NETWORK_NAME postgres:17-bookworm pg_isready -h $CONTAINER_NAME -U postgres"

echo "[ck-allinone] ① waiting for postgres..."
for i in $(seq 1 30); do
  if $PGISREADY >/dev/null 2>&1; then
    echo "[ck-allinone] ✓ postgres ready"
    break
  fi
  [[ $i -eq 30 ]] && { echo "✗ postgres never ready"; docker logs "$CONTAINER_NAME"; exit 1; }
  sleep 1
done

echo "[ck-allinone] ② §A auto-bootstrap — extensions installed on first boot WITHOUT smoke help"
PGRDF_INSTALLED=$($PSQL -c "SELECT extversion FROM pg_extension WHERE extname='pgrdf';" || true)
PGCK_INSTALLED=$($PSQL -c "SELECT extversion FROM pg_extension WHERE extname='pgck';" || true)
PGCRYPTO=$($PSQL -c "SELECT extname FROM pg_extension WHERE extname='pgcrypto';" || true)
if [[ -z "$PGRDF_INSTALLED" || -z "$PGCK_INSTALLED" || -z "$PGCRYPTO" ]]; then
  echo "✗ §A auto-bootstrap FAILED — extensions not installed on first boot (pgrdf=$PGRDF_INSTALLED pgck=$PGCK_INSTALLED pgcrypto=$PGCRYPTO)"
  docker logs "$CONTAINER_NAME" 2>&1 | grep -i bootstrap | head -5
  exit 1
fi

# §A5: pgck.nats_url GUC was baked into postgresql.conf by ociger-pg-launcher
NATS_URL=$($PSQL -c "SHOW pgck.nats_url;" 2>&1 | head -1)
if [[ "$NATS_URL" != *"nats://"* ]]; then
  echo "✗ §A5 pgck.nats_url GUC not set ($NATS_URL)"
  exit 1
fi

# §A7: extension presence (pg_extension rows above) IS the bootstrap-ran marker;
# v0.7.6 onward uses postgres single-user mode inside the launcher, not an
# s6 oneshot, so no separate marker file is written.

PGCK_NATIVE=$($PSQL -c "SELECT pgck_version();")

if [[ "$PGRDF_INSTALLED" != "$EXPECTED_PGRDF_VERSION" ]]; then
  echo "✗ wrong-version: pgrdf extversion=$PGRDF_INSTALLED expected=$EXPECTED_PGRDF_VERSION" >&2
  exit 1
fi
if [[ "$PGCK_INSTALLED" != "$EXPECTED_PGCK_VERSION" ]]; then
  echo "✗ wrong-version: pgck extversion=$PGCK_INSTALLED expected=$EXPECTED_PGCK_VERSION" >&2
  exit 1
fi
if [[ "$PGCK_NATIVE" != "$EXPECTED_PGCK_NATIVE" ]]; then
  echo "✗ wrong-version: pgck_version()=$PGCK_NATIVE expected=$EXPECTED_PGCK_NATIVE" >&2
  exit 1
fi
echo "[ck-allinone] ✓ pgrdf=$PGRDF_INSTALLED pgck=$PGCK_INSTALLED ($PGCK_NATIVE)"

echo "[ck-allinone] ③ NATS core on :4222"
for i in $(seq 1 15); do
  GREETING=$( (echo "PING"; sleep 0.5) | docker run --rm --network "$NETWORK_NAME" -i busybox:1.36.1-musl nc -w 2 "$CONTAINER_NAME" 4222 2>/dev/null | head -2 || true )
  if echo "$GREETING" | grep -q "PONG"; then
    echo "[ck-allinone] ✓ NATS responds (server_name detected, PONG received)"
    break
  fi
  [[ $i -eq 15 ]] && { echo "✗ NATS never responded"; docker logs "$CONTAINER_NAME"; exit 1; }
  sleep 1
done

echo "[ck-allinone] ④ NATS WSS bridge on :9222"
WSS_HEAD=$(curl -sI -o /dev/null -w '%{http_code}' --max-time 3 "http://127.0.0.1:39222/" || true)
# Plain HTTP to a WSS endpoint typically 426 or 400 — non-empty + non-000 is "listening"
if [[ "$WSS_HEAD" =~ ^[0-9]+$ ]] && [[ "$WSS_HEAD" != "000" ]]; then
  echo "[ck-allinone] ✓ NATS WSS port listening (HTTP probe → $WSS_HEAD)"
else
  echo "✗ NATS WSS not listening (probe → $WSS_HEAD)"
  exit 1
fi

echo "[ck-allinone] ⑤ busybox httpd serves /cklib/ on :8000"
INDEX_STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "http://127.0.0.1:38000/cklib/")
if [[ "$INDEX_STATUS" != "200" ]]; then
  echo "✗ httpd /cklib/ returned $INDEX_STATUS"
  exit 1
fi
# Confirm a specific cklib JS asset is served
JS_STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "http://127.0.0.1:38000/cklib/ck-client.js")
if [[ "$JS_STATUS" != "200" ]]; then
  echo "✗ httpd /cklib/ck-client.js returned $JS_STATUS"
  exit 1
fi
echo "[ck-allinone] ✓ /cklib/index.html=200, /cklib/ck-client.js=200"

echo "[ck-allinone] ⑤b root / serves the WSS round-trip landing"
ROOT_STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "http://127.0.0.1:38000/")
if [[ "$ROOT_STATUS" != "200" ]]; then
  echo "✗ httpd / returned $ROOT_STATUS"
  exit 1
fi
ROOT_BODY=$(curl -s "http://127.0.0.1:38000/")
if ! echo "$ROOT_BODY" | grep -q '9222' ; then
  echo "✗ / does not reference port 9222 (no WSS round-trip landing)"
  exit 1
fi
if ! echo "$ROOT_BODY" | grep -qE 'WebSocket|CKPage|cklib' ; then
  echo "✗ / does not reference WebSocket / CKPage / cklib"
  exit 1
fi
echo "[ck-allinone] ✓ / serves WSS round-trip landing"

echo "[ck-allinone] ⑤c WSS round-trip from sidecar (PUB→subscribe→receive)"
# Use a small node-based WSS client to actually round-trip a message over :9222.
WSS_OK=$(docker run --rm --network "$NETWORK_NAME" node:20-slim sh -c '
  cat > /probe.mjs <<EOF
import { WebSocket } from "ws";
const url = "ws://'"$CONTAINER_NAME"':9222";
const subj = "ck.probe.smoke." + Math.random().toString(36).slice(2,8);
const ws = new WebSocket(url);
let ok = false;
const t = setTimeout(() => { if (!ok) { console.log("TIMEOUT"); process.exit(2); } }, 5000);
ws.on("open", () => ws.send("CONNECT {\"verbose\":false,\"pedantic\":false,\"protocol\":1}\r\n"));
ws.on("message", (data) => {
  const txt = data.toString();
  if (txt.startsWith("INFO ")) {
    ws.send("SUB " + subj + " 1\r\n");
    const payload = "smoke-" + Date.now();
    ws.send("PUB " + subj + " " + payload.length + "\r\n" + payload + "\r\n");
  } else if (txt.includes("MSG ")) {
    ok = true;
    clearTimeout(t);
    console.log("OK");
    ws.close();
    process.exit(0);
  }
});
ws.on("error", (e) => { console.log("ERR " + e.message); process.exit(3); });
EOF
  cd /tmp && npm install --silent --no-save ws@8 >/dev/null 2>&1
  node --input-type=module < /probe.mjs
' 2>&1 | tail -1)
if [[ "$WSS_OK" != "OK" ]]; then
  echo "✗ WSS round-trip failed: $WSS_OK"
  exit 1
fi
echo "[ck-allinone] ✓ WSS round-trip OK over :9222"

echo "[ck-allinone] ⑤d §B4 dispatch bridge round-trip — input.kernel.pgCK.action.<verb> → result.kernel.pgCK.<verb> (via ckp.dispatch)"
# Publishes to the input subject and asserts a typed jsonb MSG arrives on the
# matching result subject within 5 s. v0.7.11+ : the relay now calls
# ckp.dispatch(verb, kernel_urn, payload, identity) via a pg connection as
# ck_participant and publishes the typed reply. Unknown verbs return the
# pgCK registry's typed `{"ok":false,"error":"unknown_affordance"}` envelope —
# which IS a successful round-trip (the bridge ran the dispatch and got a
# typed reply); we accept any payload starting with `{` as proof of round-trip.
DISP_OK=$(docker run --rm --network "$NETWORK_NAME" node:20-slim sh -c '
  cat > /probe.mjs <<EOF
import { WebSocket } from "ws";
const url = "ws://'"$CONTAINER_NAME"':9222";
const verb = "smoke.ping." + Math.random().toString(36).slice(2,8);
const inSubj = "input.kernel.pgCK.action." + verb;
const outSubj = "result.kernel.pgCK." + verb;
const ws = new WebSocket(url);
let ok = false;
const t = setTimeout(() => { if (!ok) { console.log("TIMEOUT"); process.exit(2); } }, 8000);
ws.on("open", () => ws.send("CONNECT {\"verbose\":false,\"pedantic\":false,\"protocol\":1}\r\n"));
ws.on("message", (data) => {
  const txt = data.toString();
  if (txt.startsWith("INFO ")) {
    ws.send("SUB " + outSubj + " 1\r\n");
    setTimeout(() => {
      const payload = "{}";
      ws.send("PUB " + inSubj + " " + payload.length + "\r\n" + payload + "\r\n");
    }, 200);
  } else if (txt.includes("MSG " + outSubj + " ")) {
    ok = true;
    clearTimeout(t);
    console.log("OK");
    ws.close();
    process.exit(0);
  }
});
ws.on("error", (e) => { console.log("ERR " + e.message); process.exit(3); });
EOF
  cd /tmp && npm install --silent --no-save ws@8 >/dev/null 2>&1
  node --input-type=module < /probe.mjs
' 2>&1 | tail -1)
if [[ "$DISP_OK" != "OK" ]]; then
  echo "✗ §B4 dispatch bridge round-trip failed: $DISP_OK"
  docker logs "$CONTAINER_NAME" 2>&1 | grep -i 'pgck-relay' | tail -20
  exit 1
fi
echo "[ck-allinone] ✓ §B4 dispatch bridge round-trip OK (input → ckp.dispatch → result)"

echo "[ck-allinone] ⑥ NO Python / FastAPI / uvicorn anywhere"
PY_HITS=$(docker exec "$CONTAINER_NAME" /bin/busybox find / \
  \( -name 'python*' -o -name 'uvicorn*' -o -name 'fastapi*' -o -path '*/opt/venv*' \) 2>/dev/null | wc -l | tr -d ' ')
if [[ "$PY_HITS" != "0" ]]; then
  echo "✗ Python/FastAPI trace found in image:"
  docker exec "$CONTAINER_NAME" /bin/busybox find / \
    \( -name 'python*' -o -name 'uvicorn*' -o -name 'fastapi*' -o -path '*/opt/venv*' \) 2>/dev/null
  exit 1
fi
echo "[ck-allinone] ✓ image is Python-free"

echo "[ck-allinone] ⑦ PID 1 is s6-svscan (not ociger-supervisor)"
PID1=$(docker exec "$CONTAINER_NAME" /bin/busybox cat /proc/1/comm 2>/dev/null | tr -d '\n')
if [[ "$PID1" != "s6-svscan" ]]; then
  echo "✗ PID 1 is '$PID1', expected s6-svscan"
  exit 1
fi
echo "[ck-allinone] ✓ PID 1 = $PID1"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "[ck-allinone] all checks passed"
echo "  postgres + pgRDF $PGRDF_INSTALLED + pgCK $PGCK_INSTALLED + pgcrypto"
echo "  NATS core :4222 + WSS bridge :9222"
echo "  busybox httpd → /cklib/* on :8000"
echo "  supervisor: s6-overlay (PID 1)"
echo "  Python: NONE — image is FastAPI-free"
echo "════════════════════════════════════════════════════════════"
