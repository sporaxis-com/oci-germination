#!/usr/bin/env bash
# examples/hello-kernel/run.sh
#
# A complete, runnable walk through the ck-allinone adopter journey:
#   1. stand up the published v3.9 substrate (one container)
#   2. open a domain (kernel.create)
#   3. land sealed, proof-chained state in it (task.create)
#   4. read it back + verify the proof (instances.list, instance.verify)
#   5. evolve the kernel's TYPE through consensus (propose -> vote -> apply)
#
# It needs only Docker. It uses a throwaway postgres client container as the
# participant — exactly what an app would do connecting as ck_participant —
# so there is nothing to install on the host. Every step asserts ok:true and
# the script exits non-zero on the first failure.
#
# Usage:  bash examples/hello-kernel/run.sh   [IMAGE]   [KERNEL_NAME]
#   IMAGE        default: ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.17
#   KERNEL_NAME  default: mygame
set -euo pipefail

IMAGE="${1:-ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.17}"
KERNEL="${2:-mygame}"
PW="hello-kernel-$$"
NAME="hello-kernel-$$"
NET="hello-kernel-net-$$"
CLIENT="postgres:17-bookworm"

say()  { printf '\n\033[1m%s\033[0m\n' "$*"; }
note() { printf '   %s\n' "$*"; }

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; docker network rm "$NET" >/dev/null 2>&1 || true; }
trap cleanup EXIT

# dispatch <verb> <json-payload>
#   Echoes the JSON reply to the terminal (stderr) and asserts ok:true.
#   Returns the raw reply on stdout so callers can capture it cleanly:
#     REPLY=$(dispatch verb payload)     # REPLY = just the JSON
#     dispatch verb payload >/dev/null   # reply still shown via stderr
dispatch() {
  local verb="$1" payload="$2" reply
  reply=$(docker run --rm --network "$NET" -e PGPASSWORD="$PW" "$CLIENT" \
    psql -h "$NAME" -U ck_participant -d postgres -tA -c \
    "SELECT ckp.dispatch('$verb', '$payload'::jsonb)::text;" 2>&1)
  printf '   %s\n' "$reply" >&2
  if ! printf '%s' "$reply" | grep -q '"ok": *true'; then
    printf '\033[31m   ✗ dispatch %s did not return ok:true\033[0m\n' "$verb" >&2
    docker logs "$NAME" 2>&1 | tail -20 >&2
    exit 1
  fi
  printf '%s' "$reply"
}

json() { printf '%s' "$1" | python3 -c "import json,sys; print(json.load(sys.stdin).get('$2',''))"; }

say "① Starting the substrate — $IMAGE"
docker network create "$NET" >/dev/null 2>&1 || true
docker run --rm -d --name "$NAME" --network "$NET" \
  -e OCIGER_CK_PARTICIPANT_PASSWORD="$PW" "$IMAGE" >/dev/null
note "waiting for postgres…"
for i in $(seq 1 30); do
  if docker run --rm --network "$NET" "$CLIENT" pg_isready -h "$NAME" -U postgres >/dev/null 2>&1; then
    note "ready in ${i}s"; break
  fi
  [ "$i" = 30 ] && { echo "postgres never came up"; exit 1; }
  sleep 1
done
sleep 3   # let the dispatch bridge connect

say "② Open a domain — kernel.create '$KERNEL'"
note "A 'kernel' is a named domain. Any name works; no setup needed."
dispatch kernel.create "{\"name\":\"$KERNEL\"}" >/dev/null

say "③ Land sealed state — task.create"
note "Every task.create is SHACL-gated, ledger-appended, and proof-minted in one transaction."
TASK_REPLY=$(dispatch task.create "{\"task\":{\"target_kernel\":\"$KERNEL\",\"title\":\"patrol sector 7\"}}")
TASK_ID=$(json "$TASK_REPLY" id)
PROOF=$(json "$TASK_REPLY" proof_digest)
note "sealed instance id: $TASK_ID"
note "proof digest:       ${PROOF:0:24}…"

say "④ Read it back + verify the proof"
dispatch instances.list "{\"kernel\":\"$KERNEL\"}" >/dev/null
note "instance.verify re-walks the ledger + checks the HMAC chain:"
dispatch instance.verify "{\"id\":\"$TASK_ID\"}" >/dev/null

say "⑤ Evolve the kernel's TYPE through consensus — propose → vote → apply"
note "No migration. The change is sealed as data and applies only after its quorum of votes."
PROP_REPLY=$(dispatch kernel.propose_change \
  "{\"op\":\"add_property\",\"about\":\"ckp://Kernel#$KERNEL\",\"requires_quorum\":1,\"detail\":{\"property\":\"crew_size\",\"datatype\":\"xsd:integer\"}}")
PROP_IRI=$(json "$PROP_REPLY" proposal_iri)
note "proposal: $PROP_IRI (pending)"
dispatch kernel.vote  "{\"about\":\"$PROP_IRI\",\"value\":\"approve\"}" >/dev/null
APPLY_REPLY=$(dispatch kernel.apply "{\"about\":\"$PROP_IRI\"}")
EPOCH=$(json "$APPLY_REPLY" epoch)
note "applied — kernel epoch advanced to $EPOCH"

say "✓ Done."
cat <<EOF

   What just happened, end to end, through ONE function (ckp.dispatch):
     • opened a domain                         (kernel.create)
     • landed governed, sealed, provable state (task.create → proof_digest)
     • read + independently re-verified it      (instances.list, instance.verify)
     • evolved the type by consensus            (propose → vote → apply, epoch→$EPOCH)

   The connection that did all this held exactly one capability: ckp.dispatch.
   It could not run SQL against the data, reach the query engine, or land an
   unsealed fact. That is CKP v3.9 Critical Isolation.

   Next: GETTING-STARTED.md §4 maps this onto a game / experiment / software domain.
EOF
