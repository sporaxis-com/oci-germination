#!/usr/bin/env bash
# GREEN always — SELF-CONSISTENCY, deliberately independent of versions.yaml:
# each sealed Adoption's sourceDigest must equal the sha256 of the module file
# THE IMAGE ITSELF SHIPS. init.sql computes the digest from those bytes at
# boot, so a mismatch means it sealed something other than what shipped —
# a defect on any image, never a pending re-cut.
#
# Currency (does the image carry the CURRENT upstream bytes?) is a separate
# question, asked by the digest-pin cases. Conflating the two made a stale
# image look BROKEN when it was merely old — the distinction this suite exists
# to keep. Measured 2026-08-27: v3.11 lexicon.ttl changed bytes between pgck
# 0.4.82 and 0.4.87, which is exactly the case that forced the split.
source "$(dirname "$0")/../lib.sh"

ROWS="$(QP "SELECT ckp.dispatch('instance.query','{\"type\":\"https://conceptkernel.org/ontology/v3.11/core#Adoption\"}'::jsonb)::text;")"
[ -n "$ROWS" ] || BROKEN "instance.query for Adoptions returned nothing"

for m in wave lexicon; do
  SHIPPED="$(INIMG "/bin/busybox sha256sum /ontology/v3.11/modules/$m.ttl" | awk '{print $1}')"
  [[ "$SHIPPED" =~ ^[0-9a-f]{64}$ ]] || BROKEN "cannot hash the shipped /ontology/v3.11/modules/$m.ttl (got '$SHIPPED')"
  case "$ROWS" in
    *"$SHIPPED"*) : ;;
    *) BROKEN "no sealed Adoption carries the SHIPPED $m digest $SHIPPED — init.sql sealed bytes other than the artifact's" ;;
  esac
done
GREEN "both init Adoptions seal the digests of the modules this image ships"
