#!/usr/bin/env bash
# GREEN (v0.7.41+): the ontology tree is NOT reachable over HTTP.
#
# v0.7.40 shipped `ln -s /ontology /app/ontology`, which put the whole tree in
# busybox httpd's document root and served it anonymously. It was added on
# cklib's door-confirm account — the exact dependency SPEC.CK-DOOR.v1.5.15 §3
# excludes ("NO /ontology HTTP mount is required"; law confirmation is
# wire-native via surface.grounding -> structuralDigest). Operator ruling
# 2026-08-28: no serving, no folder, no empty directory, nothing.
#
# RED (not BROKEN) when a 200 comes back: that is the pre-ruling image measured
# correctly, not a broken harness.
#
# The /ontology tree itself MUST stay on disk — init.sql reads it at boot and
# digests those bytes into the Adoptions. This case asserts the web half only,
# and asserts the disk half is intact so a "fix" that deletes the tree fails
# here instead of at boot.
source "$(dirname "$0")/../lib.sh"

CODE="$(INIMG "/bin/busybox wget -S -q -O /dev/null http://127.0.0.1:8000/ontology/v3.12/core.ttl 2>&1 | /bin/busybox awk '/HTTP\\//{print \$2; exit}'")"
[ -z "$CODE" ] && CODE=000
[ "$CODE" = "200" ] && RED "GET /ontology/v3.12/core.ttl returned 200 — the tree is served over HTTP (image predates the 2026-08-28 removal)"

INIMG "/bin/busybox test -e /app/ontology" >/dev/null 2>&1 && RED "/app/ontology exists in the httpd docroot — the symlink is still baked in"

ONDISK="$(INIMG "/bin/busybox sha256sum /ontology/v3.12/core.ttl" | awk '{print $1}')"
[ -n "$OCIGER_V312_CORE_SHA256" ] || BROKEN "versions.yaml has no v312_core_sha256 pin — nothing to consult"
[ "$ONDISK" = "$OCIGER_V312_CORE_SHA256" ] || BROKEN "/ontology/v3.12/core.ttl on disk ($ONDISK) != versions.yaml pin $OCIGER_V312_CORE_SHA256 — the boot tree must stay intact"

GREEN "ontology not served (HTTP $CODE, no /app/ontology); boot tree intact on disk (${ONDISK:0:12}…)"
