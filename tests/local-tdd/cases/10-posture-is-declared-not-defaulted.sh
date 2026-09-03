#!/usr/bin/env bash
# GREEN (v0.7.43+): pgck.admit_anonymous is OWNED by the delivery chain.
#
# Ported from the ckone bench's identity-posture-drift suite, where this was
# found. It belongs here too: that bench is somebody's machine, and a defect
# only reproducible there is one nobody else can gate. This case runs against a
# disposable container, so it travels with the repo.
#
# WHAT IT ASSERTS, and why each half matters:
#
#   1. the VALUE is emitted at all — before v0.7.43 nothing wrote the key, so it
#      sat at pgCK's built-in default `on` reading `source: default`. That is an
#      UNOWNED value: correct-looking, set by nobody, and reverting the moment
#      its accidental cause goes away.
#   2. `source` is a configuration file, NOT `default`.
#   3. `sourcefile` is NOT postgresql.auto.conf. This is the half that catches a
#      false pass: `ALTER SYSTEM` also reports source='configuration file', so an
#      operator's hand-typed statement looks identical to delivery — until the
#      next `down -v` erases it with PGDATA. The first version of this check, on
#      the bench, passed for exactly that wrong reason.
#
# The suite's container DECLARES OCIGER_CK_ADMIT_ANONYMOUS=on (see run.sh): the
# image ships CLOSED from v0.7.43 and refuses to boot without a realm, so a
# harness that wants the anonymous tier must ask for it. The assertion here is
# about OWNERSHIP, not the value — `on` is correct for this container, and
# pgCK's anonymous tier is a capability rather than a defect.
source "$(dirname "$0")/../lib.sh"

ROW="$(Q "SELECT setting||'|'||source||'|'||COALESCE(sourcefile,'(none)') FROM pg_settings WHERE name='pgck.admit_anonymous';")"
[ -n "$ROW" ] || BROKEN "could not read pgck.admit_anonymous"

VAL="${ROW%%|*}"
REST="${ROW#*|}"
SRC="${REST%%|*}"
FILE="${REST##*|}"

[ "$SRC" = "default" ] && RED "pgck.admit_anonymous=$VAL is UNOWNED (source: default) — nothing in the delivery chain emits it; image predates the v0.7.43 posture re-cut"

case "$FILE" in
  *postgresql.auto.conf)
    BROKEN "admit_anonymous is owned by $FILE — that is an ALTER SYSTEM inside PGDATA, not delivery. No test container should have one; something wrote to this container." ;;
esac

# The env the harness declared must be the value that lands. If these diverge,
# the provisioner is deriving something instead of reading what it was handed.
DECLARED="$(docker inspect "$C" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | sed -n 's/^OCIGER_CK_ADMIT_ANONYMOUS=//p' | head -1)"
[ -n "$DECLARED" ] || BROKEN "the test container declares no OCIGER_CK_ADMIT_ANONYMOUS — since v0.7.43 the harness must state the posture it is testing"
[ "$VAL" = "$DECLARED" ] || BROKEN "declared OCIGER_CK_ADMIT_ANONYMOUS=$DECLARED but the substrate runs admit_anonymous=$VAL (from $FILE) — the env is not reaching pgck.conf"

# The value must come from the fragment the provisioner regenerates at EVERY
# container start — that is what makes it survive a volume wipe.
INCONF="$(INIMG "/bin/busybox grep -c admit_anonymous /run/ck-identity/pgck.conf" 2>/dev/null | tr -d ' ')"
[ "${INCONF:-0}" -ge 1 ] || RED "admit_anonymous is not in /run/ck-identity/pgck.conf (found '${INCONF:-0}') — it is owned by something with a shorter lifetime than the provisioner"

# ── WHAT THE IMAGE SHIPS, independently of what this harness declared ──
#
# Everything above tests the RUNNING container, which deliberately declares
# `on` so the suite has a bootable anonymous door. That means none of it can
# catch the shipped default being wrong: flip the Dockerfile to ENV
# OCIGER_CK_ADMIT_ANONYMOUS=on and every assertion above still passes, because
# the harness overrides it either way.
#
# Case 11(a) covers it indirectly — a bare run must refuse, which only happens
# when the baked default is `off` — but that is inference. This asserts it
# directly, against the image rather than the container.
BAKED="$(docker image inspect "${OG_TDD_IMAGE:-ociger-ck-allinone:local-preflight}" \
  --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
  | sed -n 's/^OCIGER_CK_ADMIT_ANONYMOUS=//p' | head -1)"
[ -n "$BAKED" ] || RED "the image bakes no OCIGER_CK_ADMIT_ANONYMOUS — the default is back in code or absent; it must be visible in \`docker inspect\`"
[ "$BAKED" = "off" ] || RED "the image ships OCIGER_CK_ADMIT_ANONYMOUS=$BAKED — v0.7.43 ships the door CLOSED; \`on\` as the shipped default means an operator who configures nothing gets a door that admits everyone"

GREEN "image ships closed (baked=off); this container declares $DECLARED and the substrate runs $VAL from $FILE"
