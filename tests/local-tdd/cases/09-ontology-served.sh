#!/usr/bin/env bash
# GREEN always (v0.7.40+): the ontology tree is SERVED over HTTP, and the bytes
# served are the bytes the kernel boots from.
#
# Why this is a gate and not a nicety: this bundle ships a digest-gated ontology
# and OFFERS it over HTTP so an operator can read what THIS deployment booted
# from without shell access. It is offered, never depended on —
# SPEC.CK-DOOR.v1.5.15 §3 rules that no /ontology mount is required and that law
# confirmation is WIRE-NATIVE (`surface.grounding → structuralDigest`). Nothing
# here may be justified on a client's account; an earlier revision of this
# comment was, and is withdrawn.
#
# It asserts SAMENESS, not a constant: served bytes == /ontology bytes == the
# versions.yaml v3.12 pin. A copy that drifted from the live path would still
# serve a valid-looking root while the kernel grounded on something else — a
# confident wrong answer, which is worse than a 404.
source "$(dirname "$0")/../lib.sh"

# STATUS BEFORE HASH. A 404 body is still bytes: hashing it yields the
# empty-string digest e3b0c442… and the run then reports "served bytes differ"
# — a divergence that never existed — instead of "not served". CK.Lib.Js baked
# this exact lesson into door-confirm after their ancestor script did it; I
# reproduced it here on 2026-08-27 and am fixing it the same way.
if ! INIMG "/bin/busybox wget -q -O /dev/null http://127.0.0.1:8000/ontology/v3.12/core.ttl" >/dev/null 2>&1; then
  RED "GET /ontology/v3.12/core.ttl is not served — image predates the ontology-serving re-cut (v0.7.40)"
fi
SERVED="$(INIMG "/bin/busybox wget -qO- http://127.0.0.1:8000/ontology/v3.12/core.ttl | /bin/busybox sha256sum" | awk '{print $1}')"
[[ "$SERVED" =~ ^[0-9a-f]{64}$ ]] || BROKEN "served /ontology/v3.12/core.ttl did not hash (got '$SERVED')"

ONDISK="$(INIMG "/bin/busybox sha256sum /ontology/v3.12/core.ttl" | awk '{print $1}')"
[ "$SERVED" = "$ONDISK" ] || BROKEN "SERVED bytes ($SERVED) differ from the /ontology the kernel boots from ($ONDISK) — a consumer would confirm a root this deployment is not running"

[ -n "$OCIGER_V312_CORE_SHA256" ] || BROKEN "versions.yaml has no v312_core_sha256 pin — nothing to consult"
[ "$SERVED" = "$OCIGER_V312_CORE_SHA256" ] || BROKEN "served root $SERVED != versions.yaml pin $OCIGER_V312_CORE_SHA256"
GREEN "ontology served over HTTP; served == on-disk == versions.yaml pin (${SERVED:0:12}…)"
