#!/usr/bin/env bash
# smoke-pg17-bookworm-pgrdf.sh — proves the operable, zero-maintenance contract:
# after `docker run` with NO additional work, pgRDF is installed and ready, and
# the in-image client (psql / pg_isready on PATH) drives it via `docker exec`.
#
# This is the benchmark-runner contract: a runner that does
# `docker exec <c> psql …` must just work — no sidecar, no CREATE EXTENSION.
set -euo pipefail

IMAGE="${1:-ociger-pg17-bookworm-pgrdf:local}"
NAME="ociger-pg17-bookworm-pgrdf-smoke"
PW="smoke-bw"
# pgRDF version from versions.yaml (single source of truth) via lib/versions.sh.
source "$(dirname "${BASH_SOURCE[0]}")/lib/versions.sh"
EXPECTED_VERSION="${PGRDF_EXPECTED_VERSION:-$OCIGER_PGRDF_VERSION}"

say() { printf '[pg17-bookworm-pgrdf] %s\n' "$*"; }
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

say "starting $IMAGE (then doing NOTHING but docker exec)…"
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" -e POSTGRES_PASSWORD="$PW" "$IMAGE" >/dev/null

say "① in-image pg_isready on PATH (no sidecar)"
ready=0
for i in $(seq 1 60); do
  if docker exec "$NAME" pg_isready -U postgres >/dev/null 2>&1; then
    say "✓ ready in ${i}s (docker exec pg_isready works)"
    ready=1; break
  fi
  sleep 1
done
[[ "$ready" = 1 ]] || { echo "✗ never became ready via docker exec pg_isready"; docker logs "$NAME" | tail -20; exit 1; }

# helper: run psql in-image, return -tA output
q() { docker exec -e PGPASSWORD="$PW" "$NAME" psql -U postgres -d postgres -tA -c "$1" 2>&1; }

say "② in-image psql on PATH (the benchmark-runner driver)"
PSQL_V="$(docker exec "$NAME" psql --version 2>&1 | head -1)"
echo "$PSQL_V" | grep -q 'psql (PostgreSQL) 17' || { echo "✗ psql not usable in-image: $PSQL_V"; exit 1; }
say "✓ $PSQL_V"

say "③ pgRDF ALREADY installed at boot — ZERO manual work (no CREATE EXTENSION)"
INSTALLED="$(q "SELECT COALESCE((SELECT extversion FROM pg_extension WHERE extname='pgrdf'),'NOT CREATED');" | tail -1)"
if [[ "$INSTALLED" == "NOT CREATED" ]]; then
  echo "✗ pgrdf is NOT auto-installed — the image requires manual CREATE EXTENSION"; exit 1
fi
if [[ "$INSTALLED" != "$EXPECTED_VERSION" ]]; then
  echo "✗ wrong-version: pgrdf extversion=$INSTALLED expected=$EXPECTED_VERSION"; exit 1
fi
say "✓ pgrdf extension present at boot, extversion=$INSTALLED"

say "④ shared_preload_libraries = pgrdf (clean — no pgck noise)"
PRELOAD="$(q "SHOW shared_preload_libraries;" | tail -1)"
[[ "$PRELOAD" == "pgrdf" ]] || { echo "✗ shared_preload_libraries='$PRELOAD' (expected exactly pgrdf)"; exit 1; }
say "✓ shared_preload_libraries=pgrdf"

say "⑤ pgRDF engine works as-is: parse + store + native version()"
NATIVE="$(q "SELECT pgrdf.version();" | tail -1)"
[[ "$NATIVE" == "$EXPECTED_VERSION" ]] || { echo "✗ pgrdf.version()=$NATIVE expected=$EXPECTED_VERSION"; exit 1; }
PARSED="$(q "SELECT pgrdf.parse_turtle('@prefix e:<http://e/> . e:a e:p e:b . e:a e:q \"x\" . e:c e:p e:d .', 4242::bigint, 'urn:smoke');" | tail -1)"
[[ "$PARSED" == "3" ]] || { echo "✗ parse_turtle returned $PARSED (expected 3)"; exit 1; }
STORED="$(q "SELECT count(*) FROM pgrdf._pgrdf_quads_default WHERE graph_id=4242;" | tail -1)"
[[ "$STORED" == "3" ]] || { echo "✗ stored quads=$STORED (expected 3)"; exit 1; }
say "✓ pgrdf.version()=$NATIVE, parse_turtle 3→stored 3"

echo ""
say "════════════════════════════════════════════════════════════"
say "all checks passed — pgRDF $INSTALLED ready at boot, drivable via docker exec psql, zero maintenance"
say "════════════════════════════════════════════════════════════"
