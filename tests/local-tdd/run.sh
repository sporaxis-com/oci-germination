#!/usr/bin/env bash
# run.sh — three-state runner for oci-germination's local suite.
#
#   tests/local-tdd/run.sh [case-prefix]
#   OG_TDD_IMAGE=<ref> tests/local-tdd/run.sh      # test a candidate bundle
#
# Default image is the known-good published bundle, so an argument-less run
# reads the CURRENT ledger: GREENs that must hold everywhere, REDs that state
# the pending re-cut as predictions. When the candidate is built, point
# OG_TDD_IMAGE at it — every RED must flip GREEN, or the integration broke.
#
# Exit 0 iff no case is BROKEN (REDs-as-predicted are a pass state, reported).
# Protocol from pgCK tests/v312-tdd; see lib.sh.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
FILTER="${1:-}"

IMAGE="${OG_TDD_IMAGE:-ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.38}"
export OG_TDD_CONTAINER="${OG_TDD_CONTAINER:-og-tdd}"
export OG_TDD_NETWORK="${OG_TDD_NETWORK:-og-tdd-net}"
DATA_DIR="$ROOT/.artifacts/og-tdd/pgdata"

cleanup() {
  docker rm -f "$OG_TDD_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$OG_TDD_NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup
rm -rf "$DATA_DIR"; mkdir -p "$DATA_DIR"

echo "og-local-tdd · image: $IMAGE"
docker network create "$OG_TDD_NETWORK" >/dev/null
# Bind-mounted PGDATA on purpose: the deployment shape. og- prefix: ours.
# v0.7.43 ships the door CLOSED (OCIGER_CK_ADMIT_ANONYMOUS=off) and refuses to
# boot without a realm. This harness exercises the ANONYMOUS tier deliberately,
# so it declares that rather than inheriting it — the posture under test is now
# visible at the call site instead of being whatever the image happened to default to.
docker run -d --name "$OG_TDD_CONTAINER" --network "$OG_TDD_NETWORK" \
  -e POSTGRES_PASSWORD=ogtdd \
  -e OCIGER_CK_ADMIT_ANONYMOUS=on \
  -e OCIGER_CK_PARTICIPANT_PASSWORD=ogtdd-part \
  -v "$DATA_DIR:/var/lib/postgresql/data" \
  "$IMAGE" >/dev/null

for i in $(seq 1 45); do
  docker run --rm --network "$OG_TDD_NETWORK" postgres:18-trixie \
    pg_isready -h "$OG_TDD_CONTAINER" -U postgres >/dev/null 2>&1 && break
  [ "$i" -eq 45 ] && { echo "BROKEN: postgres never ready"; docker logs "$OG_TDD_CONTAINER" | tail -20; exit 1; }
  sleep 2
done

green=0; red=0; broken=0; brokens=""
for case_sh in "$HERE"/cases/*.sh; do
  name="$(basename "$case_sh" .sh)"
  [ -n "$FILTER" ] && [[ "$name" != "$FILTER"* ]] && continue
  out="$(bash "$case_sh" 2>&1)"; rc=$?
  case $rc in
    0)  green=$((green+1));   printf '  GREEN  %s — %s\n' "$name" "${out##*$'\n'}" ;;
    44) red=$((red+1));       printf '  RED    %s — %s\n' "$name" "${out##*$'\n'}" ;;
    *)  broken=$((broken+1)); brokens="$brokens $name"
        printf '  BROKEN %s (rc=%s)\n%s\n' "$name" "$rc" "$out" ;;
  esac
done

echo "----------------------------------------------------------------------"
echo "og-local-tdd: green $green · red-as-predicted $red · BROKEN $broken"
[ $broken -eq 0 ] || { echo "BROKEN:$brokens — stop and look."; exit 1; }
exit 0
