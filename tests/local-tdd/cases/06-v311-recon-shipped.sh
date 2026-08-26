#!/usr/bin/env bash
# RED    while the image predates the re-cut: no v3.11 recon module (0.4.82
#        added recon to BOTH trees; v3.11 core/wave/lexicon stay byte-stable).
# GREEN  when the candidate ships it at the measured digest (b2b11f1b…,
#        measured from pgck:0.4.82-pg18-nats-{arm64,amd64} on 2026-08-26).
# BROKEN on any other digest — an unvouched module, not a stale image.
source "$(dirname "$0")/../lib.sh"

V311_RECON_SHA="b2b11f1b76e22f7bfac10be3eba7b5104ff0d5d0a1d92147ec3dc392f1475d7d"
GOT="$(INIMG "/bin/busybox sha256sum /ontology/v3.11/modules/recon.ttl 2>/dev/null" | awk '{print $1}')"
[ -z "$GOT" ] && RED "no /ontology/v3.11/modules/recon.ttl — image predates the 0.4.82 re-cut"
[ "$GOT" = "$V311_RECON_SHA" ] && GREEN "v3.11 recon module shipped (b2b11f1b…)"
BROKEN "recon.ttl present with digest $GOT != measured $V311_RECON_SHA"
