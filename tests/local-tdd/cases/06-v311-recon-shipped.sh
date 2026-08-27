#!/usr/bin/env bash
# RED    while the image predates the re-cut: no v3.11 recon module (0.4.82
#        added recon to BOTH trees). NB: v3.11 was byte-stable only THROUGH
#        0.4.82 — 0.4.87 then changed lexicon.ttl; module CURRENCY is case 08's
#        question, not this one's.
# GREEN  when the candidate ships it at the measured digest (b2b11f1b…,
#        measured from pgck:0.4.82-pg18-nats-{arm64,amd64} on 2026-08-26).
# BROKEN on any other digest — an unvouched module, not a stale image.
source "$(dirname "$0")/../lib.sh"

V311_RECON_SHA="$OCIGER_RECON_SHA256"   # from versions.yaml — one source, never a second copy
GOT="$(INIMG "/bin/busybox sha256sum /ontology/v3.11/modules/recon.ttl 2>/dev/null" | awk '{print $1}')"
[ -z "$GOT" ] && RED "no /ontology/v3.11/modules/recon.ttl — image predates the 0.4.82 re-cut"
[ "$GOT" = "$V311_RECON_SHA" ] && GREEN "v3.11 recon module shipped (b2b11f1b…)"
BROKEN "recon.ttl present with digest $GOT != measured $V311_RECON_SHA"
