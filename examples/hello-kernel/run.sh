#!/usr/bin/env bash
# examples/hello-kernel/run.sh
#
# A complete, runnable walk through the ck-allinone ADOPTER journey — driven the
# way a real app or browser drives it: the CK.Lib.Js client (cklib) over
# NATS-WSS → ociger-pgck-relay → ckp.dispatch. No SQL, no postgres client.
#
#   1. activate the kernel                          (CK.activate over WSS)
#   2. land sealed, proof-chained state             (create Task → proof_digest)
#   3. read it back + independently verify          (verify, query)
#   4. prove enforcement is REAL                    (an incomplete create is rejected)
#   5. relate two instances + traverse the edge     (link → reach)
#
# It needs only Docker. It stands up the bundle, stages the bundle's OWN cklib
# (never a vendored copy — always the version that bundle ships), and runs the
# journey under node:22 (which has a native global WebSocket, so the browser
# client's ESM runs unchanged). Every step asserts; non-zero exit on first fail.
#
# Operator/debug aside — NOT the adopter surface: you can also reach the same
# door directly with psql as ck_participant (`SELECT ckp.dispatch(verb,payload)`).
# That bypasses the WSS protocol + the client and is for debugging only; the
# integration an app builds against is cklib over WSS, which is what this runs.
#
# Usage:  bash examples/hello-kernel/run.sh   [IMAGE]   [KERNEL]
#   IMAGE   default: ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.21
#   KERNEL  default: demo   (the bundle arms 'demo' with the Task/Goal shapes at boot)
set -euo pipefail

IMAGE="${1:-ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.21}"
KERNEL="${2:-demo}"
RUNNER="node:22-slim"
SUF="$$"
NAME="hello-kernel-$SUF"
NET="hello-kernel-net-$SUF"
PW="hello-kernel-$SUF"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK="$DIR/.run-$SUF"

say()  { printf '\n\033[1m%s\033[0m\n' "$*"; }
note() { printf '   %s\n' "$*"; }

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

say "① Starting the substrate — $IMAGE"
docker network create "$NET" >/dev/null 2>&1 || true
docker run -d --name "$NAME" --network "$NET" \
  -e OCIGER_CK_PARTICIPANT_PASSWORD="$PW" "$IMAGE" >/dev/null
note "waiting for postgres + the dispatch bridge…"
for i in $(seq 1 60); do
  if docker logs "$NAME" 2>&1 | grep -q "ready to accept connections"; then
    note "postgres ready in ${i}s"; break
  fi
  [ "$i" = 60 ] && { echo "postgres never came up"; docker logs "$NAME" 2>&1 | tail -20; exit 1; }
  sleep 1
done
sleep 4   # let ociger-pgck-relay connect as ck_participant and subscribe

say "② Staging the bundle's own cklib next to the driver"
mkdir -p "$WORK"
docker cp "$NAME":/app/cklib "$WORK"/cklib >/dev/null
cp "$DIR"/driver.mjs "$WORK"/driver.mjs
note "cklib: $(ls "$WORK"/cklib | tr '\n' ' ')"

say "③ Running the journey — cklib over WSS, under $RUNNER"
note "WSS endpoint: ws://$NAME:9222  (explicit — no gateway on a direct docker run)"
docker run --rm --network "$NET" \
  -e WSS="ws://$NAME:9222" -e KERNEL="$KERNEL" \
  -v "$WORK:/work:ro" "$RUNNER" node /work/driver.mjs
