#!/usr/bin/env bash
# GREEN on BOTH sides of the re-cut, deliberately: the init contract's two
# governed Adoptions (v3.11 wave + lexicon) seal digests that equal the
# versions.yaml `modules:` pins — and 0.4.82 kept all three v3.11 files
# byte-stable, so this invariant must survive the wave. If the re-cut changes
# the adoption story (v3.12 modules, recon adoption), this case is edited IN
# THE SAME COMMIT as init.sql — a pin is consulted or it is decorative.
source "$(dirname "$0")/../lib.sh"

ROWS="$(QP "SELECT ckp.dispatch('instance.query','{\"type\":\"https://conceptkernel.org/ontology/v3.11/core#Adoption\"}'::jsonb)::text;")"
for D in "$OCIGER_WAVE_SHA256" "$OCIGER_LEXICON_SHA256"; do
  [ -n "$D" ] || BROKEN "versions.yaml modules: pin missing — nothing to consult"
  case "$ROWS" in *"$D"*) : ;; *) BROKEN "no sealed Adoption carries versions.yaml digest $D" ;; esac
done
GREEN "both init Adoptions carry the versions.yaml module digests"
