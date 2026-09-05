#!/usr/bin/env bash
# RED until og#16 lands: with a project CONFIGURED, the init Adoptions are
# ORPHANED — they name `demo`, which then has no graphs, so the kernel this
# bundle actually serves composes core-only and its seals are judged by nothing.
#
# ⚠ IT MUST BOOT ITS OWN CONTAINER WITH OCIGER_CK_PROJECT SET. DO NOT
#   "SIMPLIFY" THIS TO USE THE SUITE'S CONTAINER.
# The first version of this case did exactly that and reported GREEN, because
# the suite boots with no OCIGER_CK_PROJECT: the project defaults to `demo`,
# init.sql hardcodes `demo`, they agree, and nothing is orphaned. That is
# precisely the blind spot this case exists to close — smoke ②d2 "kernel planes
# AGREE" passes for the same reason and cannot see this defect either. A case
# that only runs the default configuration cannot detect a defect that only
# appears when something is configured.
#
# IT ASKS THE SUBSTRATE. pgCK 0.4.111 added `orphaned` to fleet.adoptions for
# this shape. An earlier bench version compared roster against intoProject
# CLIENT-SIDE — a private predicate nobody else can call, unpinnable to an
# epoch, and unable to discover that it disagrees with the substrate.
#
# ⚠ PRE-0.4.111 THE CENSUS CALLED THIS HEALTHY (malformedCount 0 with both
#   Adoptions pointing at a graphless project), so a MISSING orphanedCount is
#   BROKEN, never GREEN: absence of the field is not a zero.
source "$(dirname "$0")/../lib.sh"

IMG="${OG_TDD_IMAGE:-ghcr.io/sporaxis-com/ociger-ck-allinone:latest}"
NAME="og-tdd-orphan-$$"
PROJECT="${ORPHAN_PROJECT:-orphanprobe}"
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --name "$NAME" \
  -e POSTGRES_PASSWORD=ogtdd -e OCIGER_CK_ADMIT_ANONYMOUS=on \
  -e OCIGER_CK_PROJECT="$PROJECT" "$IMG" >/dev/null 2>&1 \
  || BROKEN "could not start a probe container from $IMG"

QP() { docker exec "$NAME" /bin/busybox true 2>/dev/null; docker run --rm --network "container:$NAME" \
        -e PGPASSWORD=ogtdd postgres:18-trixie psql -h 127.0.0.1 -U postgres -tAc "$1" 2>&1 | tr -d '\r'; }
for _ in $(seq 1 60); do QP "SELECT 1" | grep -q '^1$' && break; sleep 3; done
QP "SELECT 1" | grep -q '^1$' || BROKEN "probe container never accepted connections"

PROJ="$(QP "SELECT current_setting('ckp.project',true);")"
[ "$PROJ" = "$PROJECT" ] || BROKEN "OCIGER_CK_PROJECT did not reach ckp.project (got '$PROJ') — a different finding; this case cannot speak"

OUT="$(QP "SELECT ckp.dispatch('fleet.adoptions','{}'::jsonb)::text FROM (SELECT set_config('ckp.requester','og-local-tdd',true)) _;")"
case "$OUT" in ERROR*|"") BROKEN "fleet.adoptions did not answer: ${OUT:0:160}" ;; esac

HAS="$(printf '%s' "$OUT" | python3 -c "
import json,sys
try: d=json.load(sys.stdin)
except Exception: print('parse'); raise SystemExit
print('yes' if 'orphanedCount' in d else 'no')")"
[ "$HAS" = "parse" ] && BROKEN "fleet.adoptions reply did not parse as JSON"
[ "$HAS" = "no" ]    && BROKEN "this pgCK has no orphanedCount — the census cannot answer, and its silence is NOT a zero (pre-0.4.111 it reported malformed:false for exactly this shape)"

read -r N DETAIL <<<"$(printf '%s' "$OUT" | python3 -c "
import json,sys
d=json.load(sys.stdin)
o=[a for a in (d.get('adoptions') or []) if a.get('orphaned')]
print(d.get('orphanedCount',0), '; '.join(f\"{a.get('adopts')}->{a.get('intoProject')}\" for a in o) or '-')")"

[ "${N:-0}" -eq 0 ] && GREEN "project '$PROJECT' configured and NO orphaned Adoptions — the Adoptions followed the project"
RED "project '$PROJECT' configured but orphanedCount=$N ($DETAIL) — init.sql hardcodes urn:ckp:project:demo, so the served kernel composes core-only and its seals are judged by nothing. og#16 Reset (A) is the fix; templating the name is NOT — it moves the Adoptions but germinates no ckp:Kernel, leaving a ghost"
