#!/usr/bin/env bash
# THE RE-CUT, STATED AS A PREDICTION (pgCK 0.4.82 release note: "container-
# unpacked ontologies are stale by construction until oci-germination re-cuts").
# RED    while the image predates the re-cut: no /ontology/v3.12/core.ttl.
# GREEN  when the candidate ships v3.12 FINAL at its promoted digest
#        (7de02b35…, operator ruling 2026-08-26 — RC2 bytes promoted unchanged;
#        measured from pgck:0.4.82-pg18-nats-{arm64,amd64} on 2026-08-26).
# BROKEN if a v3.12 root is present with ANY OTHER digest: an unvouched root
#        is worse than a stale one.
source "$(dirname "$0")/../lib.sh"

V312_CORE_SHA="7de02b35fd1fbc2ecfd32e6e53162704be2791a2d41280102849ddb605eb9297"
GOT="$(INIMG "/bin/busybox sha256sum /ontology/v3.12/core.ttl 2>/dev/null" | awk '{print $1}')"
[ -z "$GOT" ] && RED "no /ontology/v3.12/core.ttl — image predates the v3.12 re-cut"
[ "$GOT" = "$V312_CORE_SHA" ] && GREEN "v3.12 FINAL core shipped (7de02b35…)"
BROKEN "/ontology/v3.12/core.ttl present with digest $GOT != promoted final $V312_CORE_SHA"
