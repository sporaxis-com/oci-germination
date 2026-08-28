#!/usr/bin/env bash
# THE RE-CUT, STATED AS A PREDICTION (pgCK 0.4.82 release note: "container-
# unpacked ontologies are stale by construction until oci-germination re-cuts").
# RED    while the image predates the re-cut: no /ontology/v3.12/core.ttl.
# GREEN  when the candidate ships v3.12 FINAL at whatever digest versions.yaml
#        currently pins (re-measure on every pgck bump — a byte digest is
#        per-artifact, never carried forward). The GREEN message interpolates
#        the live pin rather than a literal: a hardcoded digest in the success
#        text goes stale the moment the pin moves, which is the same
#        decorative-vs-consulted trap this suite exists to catch — measured
#        here 2026-08-29, when a hardcoded "7de02b35…" kept printing GREEN
#        after the pin had already moved to 97f97cb2… (0.4.88).
# BROKEN if a v3.12 root is present with ANY OTHER digest: an unvouched root
#        is worse than a stale one.
source "$(dirname "$0")/../lib.sh"

V312_CORE_SHA="$OCIGER_V312_CORE_SHA256"   # from versions.yaml — one source, never a second copy
GOT="$(INIMG "/bin/busybox sha256sum /ontology/v3.12/core.ttl 2>/dev/null" | awk '{print $1}')"
[ -z "$GOT" ] && RED "no /ontology/v3.12/core.ttl — image predates the v3.12 re-cut"
[ "$GOT" = "$V312_CORE_SHA" ] && GREEN "v3.12 FINAL core shipped (${V312_CORE_SHA:0:12}…)"
BROKEN "/ontology/v3.12/core.ttl present with digest $GOT != promoted final $V312_CORE_SHA"
