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

# Component versions come from versions.yaml (single source of truth) via
# lib/versions.sh — bump versions.yaml, not this file. pgck_native is carried
# there because pgCK 0.4.13 reports a stale "0.4.3 (rc3)" natively (the
# extension is correctly 0.4.13 — pgCK NOTIFY filed). Env still overrides.
source "$(dirname "${BASH_SOURCE[0]}")/lib/versions.sh"
EXPECTED_PGRDF_VERSION="${PGRDF_EXPECTED_VERSION:-$OCIGER_PGRDF_VERSION}"
EXPECTED_PGCK_VERSION="${PGCK_EXPECTED_VERSION:-$OCIGER_PGCK_VERSION}"
EXPECTED_PGCK_NATIVE="${PGCK_EXPECTED_NATIVE_VERSION:-$OCIGER_PGCK_NATIVE}"
# cklib byte-set contract (adopted by both sides of the cklib-stale thread):
# /app/cklib MUST contain EXACTLY this file set — the attested stripped
# bundle, nothing more, nothing less. Bump together with CKLIB_VERSION in
# the Dockerfile. v1.5.0 adds ck.js (the L2 dispatch facade / entry point)
# and ck-store.js (the typed-instance cache); still no index.html/ck-page.
EXPECTED_CKLIB_FILES="${CKLIB_EXPECTED_FILES:-LICENSE README.md ck.js ck-client.js ck-store.js vendor/msgpack.js vendor/nats.ws.js}"

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
  -e OCIGER_CK_PARTICIPANT_PASSWORD=smoke-participant \
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
# Use a sidecar postgres:18-trixie (pg18 base) to connect over the smoke network.
PSQL="docker run --rm --network $NETWORK_NAME -e PGPASSWORD=smoketest postgres:18-trixie psql -h $CONTAINER_NAME -U postgres -d postgres -At -v ON_ERROR_STOP=1"
PGISREADY="docker run --rm --network $NETWORK_NAME postgres:18-trixie pg_isready -h $CONTAINER_NAME -U postgres"

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

echo "[ck-allinone] ②a pgRDF actually parses Turtle + stores quads (real round-trip, not just presence)"
# Inline Turtle fixture — 3 triples, deterministic. parse_turtle returns the
# parsed-count; we then count the stored rows in the smoke graph and assert
# both equal 3. The DELETE-before-parse keeps the assertion deterministic
# when the smoke harness re-runs against a re-used data volume locally; in
# CI the volume is always fresh.
RDF_TTL='@prefix ex: <http://example.org/> . ex:s ex:p ex:o . ex:s ex:p2 "lit" . ex:s2 ex:p ex:o2 .'
$PSQL -c "DELETE FROM pgrdf._pgrdf_quads_default WHERE graph_id = 4242;" >/dev/null 2>&1 || true
RDF_PARSED=$($PSQL -c "SELECT pgrdf.parse_turtle('$RDF_TTL', 4242::bigint, 'urn:test:smoke');" 2>&1 | tr -d ' ')
RDF_STORED=$($PSQL -c "SELECT count(*) FROM pgrdf._pgrdf_quads_default WHERE graph_id = 4242;" 2>&1 | tr -d ' ')
if [[ "$RDF_PARSED" != "3" || "$RDF_STORED" != "3" ]]; then
  echo "✗ pgRDF round-trip failed: parsed=$RDF_PARSED stored=$RDF_STORED expected both = 3"
  echo "   (this asserts pgRDF's parser AND its storage path both work end-to-end, not just that the extension installs)"
  exit 1
fi
echo "[ck-allinone] ✓ pgRDF round-trip OK (parsed=3 quads, stored=3 quads in graph_id=4242)"

echo "[ck-allinone] ②b pgCK CI-A role floor + dispatch grants (shape, not just version)"
# CI-A asserts: ck_substrate is a NOLOGIN owner, ck_participant is LOGIN-capable
# (with the deploy-supplied password), ckp.dispatch is SECURITY DEFINER, and
# ck_participant has EXECUTE on the 4-arg dispatch. Any drift here means the
# v3.9 alpha contract has silently regressed — the alpha cuts ship the floor
# automatically via CREATE EXTENSION pgck CASCADE; smoke asserts it landed.
ROLE_SUBSTRATE=$($PSQL -c "SELECT rolcanlogin FROM pg_roles WHERE rolname = 'ck_substrate';" 2>&1 | tr -d ' ')
ROLE_PARTICIPANT=$($PSQL -c "SELECT rolcanlogin FROM pg_roles WHERE rolname = 'ck_participant';" 2>&1 | tr -d ' ')
DISPATCH_SECDEF=$($PSQL -c "SELECT prosecdef FROM pg_proc WHERE proname='dispatch' AND pronamespace=(SELECT oid FROM pg_namespace WHERE nspname='ckp') LIMIT 1;" 2>&1 | tr -d ' ')
DISPATCH_GRANT=$($PSQL -c "SELECT has_function_privilege('ck_participant', 'ckp.dispatch(text,text,jsonb,text)', 'EXECUTE');" 2>&1 | tr -d ' ')
if [[ "$ROLE_SUBSTRATE" != "f" ]]; then
  echo "✗ CI-A regression: ck_substrate canlogin=$ROLE_SUBSTRATE (must be f / NOLOGIN — substrate is owner-only)"
  exit 1
fi
if [[ "$ROLE_PARTICIPANT" != "t" ]]; then
  echo "✗ CI-A regression: ck_participant canlogin=$ROLE_PARTICIPANT (must be t — needs LOGIN for external consumers)"
  exit 1
fi
if [[ "$DISPATCH_SECDEF" != "t" ]]; then
  echo "✗ CI-A regression: ckp.dispatch SECURITY DEFINER=$DISPATCH_SECDEF (must be t — the trusted-door property is core to the role floor)"
  exit 1
fi
if [[ "$DISPATCH_GRANT" != "t" ]]; then
  echo "✗ CI-A regression: ck_participant EXECUTE on ckp.dispatch=$DISPATCH_GRANT (must be t — external consumers must be able to call the door)"
  exit 1
fi
echo "[ck-allinone] ✓ CI-A floor OK (ck_substrate NOLOGIN, ck_participant LOGIN, ckp.dispatch SECURITY DEFINER + granted to ck_participant)"

echo "[ck-allinone] ②c pgCK CI-B affordance registry is populated (not a stub)"
# CI-B asserts the registry table is seeded with verbs by the extension's
# bootstrap migrations. A drift to zero rows means CI-B regressed silently.
REGISTRY_COUNT=$($PSQL -c "SELECT count(*) FROM ckp.affordance_registry;" 2>&1 | tr -d ' ')
if ! [[ "$REGISTRY_COUNT" =~ ^[0-9]+$ ]] || [[ "$REGISTRY_COUNT" -lt 10 ]]; then
  echo "✗ CI-B regression: ckp.affordance_registry has $REGISTRY_COUNT entries (expect >= 10 — pgCK v0.4.x seeds ~20 verbs)"
  exit 1
fi
echo "[ck-allinone] ✓ CI-B registry seeded ($REGISTRY_COUNT verbs)"

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

echo "[ck-allinone] ⑤ busybox httpd serves the cklib client on :8000"
# cklib 1.5.0 ships NO index.html (the stripped set), so the gate is the
# L2 facade entry point (ck.js) + the client + the vendored transport deps.
for asset in ck.js ck-client.js ck-store.js vendor/nats.ws.js vendor/msgpack.js; do
  STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "http://127.0.0.1:38000/cklib/${asset}")
  if [[ "$STATUS" != "200" ]]; then
    echo "✗ httpd /cklib/${asset} returned $STATUS"
    exit 1
  fi
done
echo "[ck-allinone] ✓ /cklib/{ck.js,ck-client.js,ck-store.js} + vendor/{nats.ws,msgpack}.js all 200"

echo "[ck-allinone] ⑤± cklib byte-set gate — /app/cklib is EXACTLY the attested stripped bundle"
# The packaging contract from the cklib-stale thread: every file under
# /app/cklib must come from the attested ck-lib-js bundle, and the attested
# bundle's whole file set must be present. Surplus files (e.g. the pre-strip
# RDF tier reappearing) or missing files both fail. Compare sorted listings.
# The image's busybox is a single static binary without applet symlinks, so
# call the find applet explicitly and normalize host-side.
ACTUAL_CKLIB_FILES=$(docker exec "$CONTAINER_NAME" /bin/busybox find /app/cklib -type f | sed 's|^/app/cklib/||' | sort | tr '\n' ' ' | sed 's/ $//')
EXPECTED_SORTED=$(echo "$EXPECTED_CKLIB_FILES" | tr ' ' '\n' | sort | tr '\n' ' ' | sed 's/ $//')
if [[ "$ACTUAL_CKLIB_FILES" != "$EXPECTED_SORTED" ]]; then
  echo "✗ cklib byte-set mismatch (stale/pre-strip surface or missing files)"
  echo "   expected: $EXPECTED_SORTED"
  echo "   actual:   $ACTUAL_CKLIB_FILES"
  exit 1
fi
echo "[ck-allinone] ✓ /app/cklib byte-set matches the attested stripped bundle ($EXPECTED_SORTED)"

echo "[ck-allinone] ⑤+ reserved web surfaces — /web/, /web2/, /wss/ all served"
# Every advertised surface MUST be present + served. A missing path silently
# breaks downstream consumers in topologies the smoke didn't exercise; the
# v0.7.5..v0.7.12 cuts shipped without these surfaces and only an envoy-fronted
# downstream discovered the gap. See feedback_static_surface_completeness.md.
for surface in web web2 wss; do
  STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "http://127.0.0.1:38000/${surface}/")
  if [[ "$STATUS" != "200" ]]; then
    echo "✗ httpd /${surface}/ returned $STATUS (reserved surface missing or broken)"
    exit 1
  fi
  # Sanity: the page must self-identify so a future regression that serves the
  # wrong content still fails the smoke.
  BODY=$(curl -s "http://127.0.0.1:38000/${surface}/")
  if ! echo "$BODY" | grep -qE "/${surface}/" ; then
    echo "✗ /${surface}/ served HTML doesn't self-identify (suspect wrong content at this path)"
    exit 1
  fi
done
echo "[ck-allinone] ✓ /web/=200, /web2/=200, /wss/=200 (all reserved surfaces present)"

echo "[ck-allinone] ⑤b root / serves the WSS round-trip landing"
ROOT_STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "http://127.0.0.1:38000/")
if [[ "$ROOT_STATUS" != "200" ]]; then
  echo "✗ httpd / returned $ROOT_STATUS"
  exit 1
fi
ROOT_BODY=$(curl -s "http://127.0.0.1:38000/")
# Assert BOTH topology branches are present in the JS — the gateway-aware
# branch (wss://host/wss for https) AND the direct-port branch (ws://host:9222
# for the docker-run posture). v0.7.5..v0.7.12 shipped a direct-port-only
# probe that broke every envoy-fronted consumer; this smoke catches that class
# of regression. See feedback_static_surface_completeness.md.
if ! echo "$ROOT_BODY" | grep -q '/wss' ; then
  echo "✗ / landing JS missing the gateway-aware /wss branch (direct-port-only regression)"
  exit 1
fi
if ! echo "$ROOT_BODY" | grep -q '9222' ; then
  echo "✗ / landing JS missing the direct-port :9222 branch"
  exit 1
fi
if ! echo "$ROOT_BODY" | grep -qE 'WebSocket|CKPage|cklib' ; then
  echo "✗ / does not reference WebSocket / CKPage / cklib"
  exit 1
fi
echo "[ck-allinone] ✓ / serves WSS round-trip landing (both topology branches present)"

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

echo "[ck-allinone] ②d two-Adoption pin ledger armed at FIRST BOOT (the 0.6.33/0.4.77 init contract)"
# v3.11 contract: init.sql runs CALL ckp.boot() then seals TWO governed
# core#Adoption instances (wave + lexicon) whose sourceDigest is computed from
# the artifact-shipped module TTLs. This gate is the CONSUMER half of pgCK's
# own fresh-install gate (3): it must (a) find both module graphs loaded,
# (b) find ckp.adoption_pins existing at install (the 0.4.76 defect: it was
# bootstrap/migration-only, so the SECOND Adoption died on every fresh
# install — and the old gate passed anyway because it never sealed two), and
# (c) hear fleet.adoptions ANSWER as ck_participant, carrying BOTH digests
# versions.yaml pins. (c) is what makes the versions.yaml `modules:` pin
# CONSULTED rather than decorative: what actually sealed at boot is compared
# byte-for-byte against the declared expectation.
PSQL_PART="docker run --rm --network $NETWORK_NAME -e PGPASSWORD=smoke-participant postgres:18-trixie psql -h $CONTAINER_NAME -U ck_participant -d postgres -At -v ON_ERROR_STOP=1"
MOD_GRAPHS=$($PSQL -c "SELECT COUNT(*) FROM pgrdf._pgrdf_graphs WHERE iri IN ('urn:ckp:module:wave','urn:ckp:module:lexicon');" 2>&1 | tr -d ' ')
if [[ "$MOD_GRAPHS" != "2" ]]; then
  echo "✗ module graphs not loaded at first boot (found $MOD_GRAPHS of 2) — init.sql load_turtle didn't run or /ontology/v3.11/modules/* missing from pg_base"
  docker logs "$CONTAINER_NAME" 2>&1 | grep -iE 'ontology|load_turtle|ERROR' | head -5
  exit 1
fi
PINS_TABLE=$($PSQL -c "SELECT (to_regclass('ckp.adoption_pins') IS NOT NULL)::text;" 2>&1 | tr -d ' ')
if [[ "$PINS_TABLE" != "true" ]]; then
  echo "✗ ckp.adoption_pins missing after CREATE EXTENSION — the pin ledger is still bootstrap/migration-only (pre-0.4.77 defect shape)"
  exit 1
fi
# Composition health: fleet.adoptions answers, both modules listed, zero
# malformed (its own malformed:true class = an Adoption whose adopts IRI names
# no non-empty graph — a judged Adoption composing NOTHING).
FLEET_OUT=$($PSQL_PART -c "SELECT ckp.dispatch('fleet.adoptions','{}'::jsonb)::text;" 2>&1)
if [[ "$FLEET_OUT" != *'"ok": true'* && "$FLEET_OUT" != *'"ok":true'* ]]; then
  echo "✗ fleet.adoptions did not answer ok:true on a fresh install: ${FLEET_OUT:0:240}"
  exit 1
fi
for M in "urn:ckp:module:wave" "urn:ckp:module:lexicon"; do
  if [[ "$FLEET_OUT" != *"$M"* ]]; then
    echo "✗ fleet.adoptions does not list $M: ${FLEET_OUT:0:400}"
    exit 1
  fi
done
if [[ "$FLEET_OUT" != *'"malformedCount": 0'* && "$FLEET_OUT" != *'"malformedCount":0'* ]]; then
  echo "✗ fleet.adoptions reports malformed adoptions (an Adoption composing NOTHING): ${FLEET_OUT:0:400}"
  exit 1
fi
# The consulted pin: fleet.adoptions does NOT render sourceDigest (it shows
# structuralPin), so the byte-digest comparison reads the sealed Adoption
# instances through the door. Each must carry EXACTLY the digest versions.yaml
# declares for its module — this is where a carried pin becomes a consulted one.
ADOPT_ROWS=$($PSQL_PART -c "SELECT ckp.dispatch('instance.query','{\"type\":\"https://conceptkernel.org/ontology/v3.11/core#Adoption\"}'::jsonb)::text;" 2>&1)
for D in "$OCIGER_WAVE_SHA256" "$OCIGER_LEXICON_SHA256"; do
  if [[ "$ADOPT_ROWS" != *"$D"* ]]; then
    echo "✗ pin NOT consulted: no sealed Adoption carries versions.yaml digest $D"
    echo "   instance.query Adoption: ${ADOPT_ROWS:0:400}"
    exit 1
  fi
done
echo "[ck-allinone] ✓ pin ledger armed: 2 module graphs, adoption_pins at install, fleet.adoptions clean, sealed sourceDigests match versions.yaml"

echo "[ck-allinone] ②e v3.9 floor — ck_participant reaches NOTHING beyond ckp.dispatch"
# The Critical Isolation contract (SPEC.CKP.v3.9 §7 / P8): participant
# credentials confer dispatch, nothing more. In particular NO reach into
# schema pgrdf — the v0.7.14..v0.7.16 init.sql granted pgrdf to
# ck_participant as a workaround for a pgCK 0.4.1 install gap; pgCK 0.4.2
# removed the need and flagged the grant as a floor breach. This gate makes
# the breach class unshippable.
PART_PGRDF=$($PSQL -c "SELECT has_schema_privilege('ck_participant','pgrdf','USAGE');" 2>&1 | tr -d ' ')
if [[ "$PART_PGRDF" != "f" ]]; then
  echo "✗ v3.9 floor breach: ck_participant has USAGE on schema pgrdf (must be f)"
  exit 1
fi
echo "[ck-allinone] ✓ floor intact: ck_participant has no pgrdf reach"

echo "[ck-allinone] ⑤d §B4 dispatch bridge round-trip + FAIL-CLOSED refusal (typed reply, no anonymous seal)"
# 0.4.77 contract REVERSAL of the old semantic pass: this bundle boots with
# NO kernel loaded (boot + two module Adoptions only), and 0.4.64+ refuses
# unattributed seals — so an anonymous task.create arriving over the NATS
# bridge must be REFUSED, cleanly, with a typed reply. pgCK's own fresh-install
# gate asserts exactly this ("board verb refused with no kernel loaded — R2
# holds on a virgin substrate"); a governed ok:true here would be the OLD #46
# vacuous-allowance escape, shipping an anonymous write path.
# What this gate still proves POSITIVELY: the in-extension inbound dispatch is
# alive end-to-end — PUB input.kernel.pgck.action.<verb> → a typed
# result.kernel.pgck.<verb> reply — and the reply is a real refusal, not the
# 4-arg CI-A-2 stub ("verb not governed yet") and not a relation-missing crash
# (the v0.7.14 escape class). The GOVERNED-SEAL positive proof moved to ⑤g,
# where an ATTRIBUTED participant seals a v3.11-root type through the door.
DISP_OUT=$(docker run --rm --network "$NETWORK_NAME" node:20-slim sh -c '
  cat > /probe.mjs <<EOF
import { WebSocket } from "ws";
const url = "ws://'"$CONTAINER_NAME"':9222";
const verb = "task.create";
const inSubj = "input.kernel.pgck.action." + verb;
const outSubj = "result.kernel.pgck." + verb;
const ws = new WebSocket(url);
const t = setTimeout(() => { console.log("FAIL timeout"); process.exit(2); }, 8000);
ws.on("open", () => ws.send("CONNECT {\"verbose\":false,\"pedantic\":false,\"protocol\":1}\r\n"));
ws.on("message", (data) => {
  const txt = data.toString();
  if (txt.startsWith("INFO ")) {
    ws.send("SUB " + outSubj + " 1\r\n");
    setTimeout(() => {
      const payload = JSON.stringify({task:{target_kernel:"demo",title:"smoke check"}});
      ws.send("PUB " + inSubj + " " + payload.length + "\r\n" + payload + "\r\n");
    }, 200);
  } else if (txt.startsWith("MSG ")) {
    clearTimeout(t);
    const parts = txt.split("\r\n");
    const body = parts[1] || "";
    let env;
    try { env = JSON.parse(body); }
    catch(e) { console.log("FAIL not-json:" + body.slice(0,160)); process.exit(3); }

    var hasDelegate = Object.prototype.hasOwnProperty.call(env, "delegate");
    var isStubError = /not governed yet/.test(String(env.error||""));
    if (hasDelegate || isStubError) {
      console.log("FAIL called-4-arg-stub must-call-GOVERNED-2-arg body=" + body.slice(0,240));
      process.exit(10);
    }
    if (/does not exist/.test(String(env.error||""))) {
      console.log("FAIL relation-missing — init.sql bootstrap did not run; this is the v0.7.14 escape class. body=" + body.slice(0,240));
      process.exit(12);
    }
    if (env.ok === true) {
      console.log("FAIL anonymous-seal-ACCEPTED — an unattributed board verb sealed on a kernel-less fresh install (fail-closed breached). body=" + body.slice(0,240));
      process.exit(13);
    }
    if (env.ok === false) {
      console.log("OK refused-fail-closed error=" + String(env.error||"").slice(0,120));
      ws.close();
      process.exit(0);
    }
    console.log("FAIL untyped-reply body=" + body.slice(0,240));
    process.exit(14);
  }
});
ws.on("error", (e) => { console.log("FAIL ws-error:" + e.message); process.exit(7); });
EOF
  cd /tmp && npm install --silent --no-save ws@8 >/dev/null 2>&1
  node --input-type=module < /probe.mjs
' 2>&1 | tail -1)
if [[ "$DISP_OUT" != OK* ]]; then
  echo "✗ §B4 dispatch bridge GOVERNED-seal round-trip failed: $DISP_OUT"
  docker logs "$CONTAINER_NAME" 2>&1 | grep -iE 'pgck|nats|dispatch' | tail -20
  exit 1
fi
echo "[ck-allinone] ✓ §B4 dispatch bridge GOVERNED seal round-trip ($DISP_OUT)"

echo "[ck-allinone] ⑤d2 §B4b forge-deny — a client CANNOT publish a governed *.sealed event"
# v0.7.30 INTERIM mitigation (pending the auth-callout wiring, og#19). An
# anonymous connection may still publish to input.*; without this deny a client
# could PUB input.kernel.pgck.action.Task.sealed and forge a sealed fact (+ by:
# header). nats-server.conf maps the anon connection to a deny of that exact
# pattern. SUPERSEDED once pgck.nats_account_seed is delivered — the auth-callout
# anon tier is subscribe-only (no publish at all), a strictly stronger deny.
# The legit result.kernel round-trip above proves non-.sealed still flows.
FORGE_OUT=$(docker run --rm --network "$NETWORK_NAME" node:20-slim sh -c '
  mkdir /w && cd /w && npm init -y >/dev/null 2>&1 && npm i ws >/dev/null 2>&1
  cat > /w/f.mjs <<EOF
import { WebSocket } from "ws";
const ws = new WebSocket("ws://'"$CONTAINER_NAME"':9222");
const t = setTimeout(() => { console.log("FAIL not-denied"); process.exit(2); }, 6000);
ws.on("open", () => ws.send("CONNECT {\"verbose\":false,\"protocol\":1}\r\n"));
let s = false;
ws.on("message", (d) => { const x = d.toString();
  if (x.startsWith("INFO ") && !s) { s = true;
    const p = "{\"forged\":true}";
    ws.send("PUB input.kernel.pgck.action.Task.sealed " + p.length + "\r\n" + p + "\r\n"); }
  else if (x.startsWith("-ERR") && /Permissions Violation/.test(x)) { clearTimeout(t); console.log("DENIED"); process.exit(0); } });
EOF
  node /w/f.mjs' 2>&1 | tail -1)
if [[ "$FORGE_OUT" != "DENIED" ]]; then
  echo "✗ §B4b forge-deny FAILED — client could forge a *.sealed event: $FORGE_OUT" >&2
  exit 1
fi
echo "[ck-allinone] ✓ §B4b forge-deny enforced (Permissions Violation on input.kernel.pgck.action.*.sealed)"

echo "[ck-allinone] ⑤e governance plane — propose → vote → apply advances the kernel epoch (CKP v3.9 §5)"
# The adoption-critical path: a participant (not superuser) governs a type
# change in through the sealed door. Asserts the FULL lifecycle: Proposal
# sealed {pending} → Vote sealed, quorum met → apply flips state to
# {applied} and bumps the kernel epoch. This is the regression trap for
# the v3.9 governance plane end-to-end — if any of registry routing, the
# ProposalShape gate, the quorum count, or the apply cascade regresses,
# this fails before a release ships.
# Payload contract (pgCK 0.4.5+, verified live on 0.4.13): add_property
# requires detail.targetClass + detail.path as IRIs — v3.11 IRIs now; the old
# v3.8 core#Task target names a type the v3.11 root retired outright.
# 0.4.64+: every dispatch DECLARES its requester via set_config('ckp.requester',
# …) in the same statement — unattributed seals refuse (that refusal is gate
# ⑤d's business; this gate exercises the attributed path).
EPOCH_BEFORE=$($PSQL -c "SELECT COALESCE(MAX(epoch),1) FROM ckp.kernel_epoch;" 2>/dev/null | tr -d ' ')
GOV_PROPOSE=$($PSQL_PART -c "SELECT ckp.dispatch('kernel.propose_change', '{\"op\":\"add_property\",\"requires_quorum\":1,\"detail\":{\"targetClass\":\"https://conceptkernel.org/ontology/v3.11/core#Supersession\",\"path\":\"https://conceptkernel.org/ontology/v3.11/core#smokeProp\",\"datatype\":\"xsd:string\",\"minCount\":1}}'::jsonb)::text FROM (SELECT set_config('ckp.requester','smoke-participant',true)) _id;" 2>&1)
GOV_IRI=$(echo "$GOV_PROPOSE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('proposal_iri',''))" 2>/dev/null)
if [[ -z "$GOV_IRI" ]]; then
  echo "✗ governance propose failed: $GOV_PROPOSE"
  exit 1
fi
GOV_VOTE=$($PSQL_PART -c "SELECT ckp.dispatch('kernel.vote', '{\"about\":\"$GOV_IRI\",\"value\":\"approve\"}'::jsonb)::text FROM (SELECT set_config('ckp.requester','smoke-participant',true)) _id;" 2>&1)
GOV_QUORUM=$(echo "$GOV_VOTE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('quorum_met',False))" 2>/dev/null)
if [[ "$GOV_QUORUM" != "True" ]]; then
  echo "✗ governance vote failed or quorum not met: $GOV_VOTE"
  exit 1
fi
GOV_APPLY=$($PSQL_PART -c "SELECT ckp.dispatch('kernel.apply', '{\"about\":\"$GOV_IRI\"}'::jsonb)::text FROM (SELECT set_config('ckp.requester','smoke-participant',true)) _id;" 2>&1)
GOV_STATE=$(echo "$GOV_APPLY" | python3 -c "import json,sys; print(json.load(sys.stdin).get('state',''))" 2>/dev/null)
EPOCH_AFTER=$($PSQL -c "SELECT COALESCE(MAX(epoch),1) FROM ckp.kernel_epoch;" 2>/dev/null | tr -d ' ')
if [[ "$GOV_STATE" != "applied" ]]; then
  echo "✗ governance apply failed: $GOV_APPLY"
  exit 1
fi
if [[ "$EPOCH_AFTER" -le "$EPOCH_BEFORE" ]]; then
  echo "✗ governance apply did not advance the kernel epoch (before=$EPOCH_BEFORE after=$EPOCH_AFTER)"
  exit 1
fi
echo "[ck-allinone] ✓ governance plane OK (proposal sealed → quorum met → applied; epoch $EPOCH_BEFORE → $EPOCH_AFTER)"

echo "[ck-allinone] ⑤f native outbox drain — a seal emits event.kernel.pgck.<class>.sealed (NO host bridge)"
# v0.7.18+ : every seal enqueues a ckp.outbox row; pgCK's in-.so native drain
# publishes it onto event.kernel.<K>.<class>.sealed. This gate proves the
# bundle moves sealed events onto NATS BY ITSELF — the regression class
# CK.Lib.Js's no-native-outbox-drain NOTIFY surfaced (their verify only passed
# with a stray host-side dev drain).
# 0.4.77 rework: the old trigger (anonymous task.create over WSS) now REFUSES
# by design (⑤d), so a refusal would never reach the outbox. The trigger is
# now an ATTRIBUTED participant seal of a complete wave#Finding (adopted at
# boot; requires label + reason + findingState) fired via SQL while a detached
# subscriber listens on event.>. NOT core#Supersession: gate ⑤e's applied
# governance change adds a required smokeProp to that class, and the applied
# constraint is ENFORCED — a bare Supersession refuses after ⑤e (measured;
# the refusal is ⑤e's proof working, not a defect). Subscribe-first ordering
# is preserved by the two-phase start: the probe prints SUBSCRIBED, the host
# waits for it, then seals.
PROBE_NAME="$CONTAINER_NAME-drainprobe"
docker rm -f "$PROBE_NAME" >/dev/null 2>&1 || true
docker run -d --name "$PROBE_NAME" --network "$NETWORK_NAME" node:20-slim sh -c '
  cat > /probe.mjs <<EOF
import { WebSocket } from "ws";
const ws = new WebSocket("ws://'"$CONTAINER_NAME"':9222");
let subscribed = false;
const t = setTimeout(() => { console.log("FAIL no-event-in-30s"); process.exit(2); }, 30000);
ws.on("open", () => ws.send("CONNECT {\"verbose\":false,\"pedantic\":false,\"protocol\":1}\r\n"));
ws.on("message", (data) => {
  const txt = data.toString();
  if (txt.startsWith("INFO ")) {
    ws.send("SUB event.> 1\r\n");
    subscribed = true;
    console.log("SUBSCRIBED");
  } else if (subscribed && txt.includes("MSG event.")) {
    clearTimeout(t);
    const subj = txt.split("\r\n")[0].split(" ")[1];
    console.log("OK " + subj);
    ws.close();
    process.exit(0);
  }
});
ws.on("error", (e) => { console.log("FAIL ws-error:" + e.message); process.exit(3); });
EOF
  cd /tmp && npm install --silent --no-save ws@8 >/dev/null 2>&1
  node --input-type=module < /probe.mjs
' >/dev/null
for i in $(seq 1 20); do
  docker logs "$PROBE_NAME" 2>/dev/null | grep -q SUBSCRIBED && break
  sleep 1
done
SEAL_TRIGGER=$($PSQL_PART -c "SELECT ckp.dispatch('instance.create','{\"type\":\"https://conceptkernel.org/ontology/v3.11/wave#Finding\",\"label\":\"smoke drain probe\",\"reason\":\"ck-allinone smoke: trigger seal for the native outbox drain gate\",\"findingState\":\"open\"}'::jsonb)->>'ok' FROM (SELECT set_config('ckp.requester','smoke-participant',true)) _id;" 2>&1 | tr -d ' ')
if [[ "$SEAL_TRIGGER" != "true" ]]; then
  docker rm -f "$PROBE_NAME" >/dev/null 2>&1 || true
  echo "✗ drain-probe trigger seal refused/errored (ok=$SEAL_TRIGGER) — attributed participant instance.create of a complete wave#Finding must seal"
  exit 1
fi
DRAIN_OK=$(docker wait "$PROBE_NAME" >/dev/null 2>&1; docker logs "$PROBE_NAME" 2>&1 | tail -1)
docker rm -f "$PROBE_NAME" >/dev/null 2>&1 || true
if [[ "$DRAIN_OK" != OK* ]]; then
  echo "✗ native outbox drain failed: $DRAIN_OK"
  echo "   (a seal did not emit an event on event.kernel.pgck.* — pgCK's in-.so drain isn't running)"
  docker logs "$CONTAINER_NAME" 2>&1 | grep -iE 'pgck|nats|drain' | tail -15
  exit 1
fi
echo "[ck-allinone] ✓ native drain OK — seal → $DRAIN_OK (no host bridge)"

echo "[ck-allinone] ⑤g typed-edge enforcement is NON-VACUOUS on the ADOPTED surface (#18 class, v3.11 form)"
# The #18 lesson, re-derived for v3.11: a presence check can mask a wrong-graph
# silent pass, so this gate asserts BEHAVIOR through the door as ck_participant.
# The two init.sql Adoptions composed wave/lexicon into the surface (the
# fresh-install walk measures 42 NodeShapes = 27 core + 11 wave + 4 lexicon);
# if the composition silently failed, (b) would MINT instead of refuse and (c)
# would refuse instead of seal — either way this gate fails.
# (a) shape floor: >=42 NodeShapes present across graphs after boot+adoptions.
# (b) an INCOMPLETE wave#Finding (FindingShape requires label + reason +
#     findingState) must be REFUSED — the adopted module's shapes JUDGE.
# (c) a COMPLETE wave#Finding must SEAL with a proof_digest — the positive
#     governed-seal proof that ⑤d used to carry (attributed, so it seals).
SH_ALL=$($PSQL -c "SELECT count(*) FROM pgrdf.sparql('PREFIX sh: <http://www.w3.org/ns/shacl#> SELECT ?s WHERE { GRAPH ?g { ?s a sh:NodeShape } }') j;" 2>&1 | tr -d ' ')
if ! [[ "$SH_ALL" =~ ^[0-9]+$ ]] || [[ "$SH_ALL" -lt 42 ]]; then
  echo "✗ shape floor broken: $SH_ALL NodeShapes across graphs (expect >=42 = 27 core + 11 wave + 4 lexicon) — boot or the two Adoptions did not compose"
  exit 1
fi
TE_INCOMPLETE=$($PSQL_PART -c "SELECT ckp.dispatch('instance.create','{\"type\":\"https://conceptkernel.org/ontology/v3.11/wave#Finding\",\"label\":\"smoke incomplete\"}'::jsonb)->>'ok' FROM (SELECT set_config('ckp.requester','smoke-participant',true)) _id;" 2>&1 | tr -d ' ')
if [[ "$TE_INCOMPLETE" != "false" ]]; then
  echo "✗ typed-edge gate VACUOUS: an incomplete wave#Finding returned ok=$TE_INCOMPLETE (expect false) — the adopted shapes are not judging (the #18 silent-pass, adoption form)"
  exit 1
fi
TE_COMPLETE=$($PSQL_PART -c "SELECT (d->>'ok') || ':' || COALESCE(substr(d->>'proof_digest',1,12),'NO-PROOF') FROM (SELECT ckp.dispatch('instance.create','{\"type\":\"https://conceptkernel.org/ontology/v3.11/wave#Finding\",\"label\":\"smoke governed seal\",\"reason\":\"ck-allinone smoke: attributed participant seal through the door on a fresh install\",\"findingState\":\"open\"}'::jsonb) AS d FROM (SELECT set_config('ckp.requester','smoke-participant',true)) _id) _;" 2>&1 | tr -d ' ')
if [[ "$TE_COMPLETE" != true:* ]] || [[ "$TE_COMPLETE" == *NO-PROOF* ]]; then
  echo "✗ governed seal failed: complete wave#Finding returned $TE_COMPLETE (expect true:<proof_digest>)"
  exit 1
fi
echo "[ck-allinone] ✓ typed-edge non-vacuous on adopted surface ($SH_ALL NodeShapes; incomplete Finding REFUSED; complete Finding SEALED $TE_COMPLETE)"

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

echo "[ck-allinone] ⑧ privilege gate — exposed longruns NON-root + seed not world-readable (v0.7.31 hardening, AUDIT.CK-ALLINONE)"
# Reads /proc via the image's /bin/sh (shell builtins + cat only — no grep/awk in
# the image); parses on the host. Asserts nats-server + busybox httpd do NOT run
# as uid 0, and that the account-seed file is 0640 owned by postgres(999) so the
# dropped-to-non-root services cannot read it.
PROC=$(docker exec "$CONTAINER_NAME" /bin/sh -c '
for p in /proc/[0-9]*; do
  [ -r "$p/comm" ] || continue
  comm=$(cat "$p/comm" 2>/dev/null)
  uid=""
  while read k v _rest; do [ "$k" = "Uid:" ] && { uid=$v; break; }; done < "$p/status" 2>/dev/null
  cmd=$(cat "$p/cmdline" 2>/dev/null)
  echo "$comm|$uid|$cmd"
done' 2>/dev/null)
if echo "$PROC" | awk -F"|" '$1=="nats-server" && $2=="0"{f=1} END{exit !f}'; then
  echo "✗ nats-server runs as ROOT (uid 0) — R1 regression"; echo "$PROC" | grep "^nats-server"; exit 1
fi
if echo "$PROC" | awk -F"|" '$3 ~ /httpd/ && $2=="0"{f=1} END{exit !f}'; then
  echo "✗ busybox httpd runs as ROOT (uid 0) — R2 regression"; echo "$PROC" | grep "httpd" | head -1; exit 1
fi
SEED=$(docker exec "$CONTAINER_NAME" /bin/busybox ls -ln /run/ck-identity/pgck.conf 2>/dev/null)
SPERM=$(echo "$SEED" | awk '{print $1}'); SUID=$(echo "$SEED" | awk '{print $3}')
if [[ -n "$SEED" ]]; then
  if [[ "$(printf '%s' "$SPERM" | cut -c8)" == "r" ]]; then
    echo "✗ seed file pgck.conf is world/other-readable ($SPERM) — R3 regression"; exit 1
  fi
  if [[ "$SUID" != "999" ]]; then
    echo "✗ seed file pgck.conf not owned by postgres(999): uid=$SUID — R3 regression"; exit 1
  fi
fi
echo "[ck-allinone] ✓ nats-server + httpd NON-root; seed pgck.conf ${SPERM:-<none>} owner ${SUID:-?} (postgres-only)"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "[ck-allinone] all checks passed"
echo "  postgres + pgRDF $PGRDF_INSTALLED + pgCK $PGCK_INSTALLED + pgcrypto"
echo "  NATS core :4222 + WSS bridge :9222"
echo "  busybox httpd → /cklib/* on :8000"
echo "  supervisor: s6-overlay (PID 1)"
echo "  Python: NONE — image is FastAPI-free"
echo "════════════════════════════════════════════════════════════"
