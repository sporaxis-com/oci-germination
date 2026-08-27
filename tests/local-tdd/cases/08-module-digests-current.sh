#!/usr/bin/env bash
# CURRENCY: does this image carry the module bytes versions.yaml pins?
#
# GREEN  every v3.11 module file hashes to its versions.yaml pin.
# RED    a file hashes to a PREVIOUSLY SHIPPED digest — the image predates the
#        re-cut. The previous digests are named explicitly below, so "stale"
#        must be proven, never inferred from "not current".
# BROKEN any other digest: an unvouched module. Neither current nor a known
#        ancestor is the one case that must stop the line.
#
# Why this case exists: pgck 0.4.87 changed v3.11 lexicon.ttl (ce9f20f4… →
# 5def86ba…) while core/wave/recon and the whole v3.12 tree stayed byte-stable.
# Sealed Adoptions cite these bytes, so a silent drift here would re-point the
# adoption pin at bytes nobody vouched for. Measured 2026-08-27.
source "$(dirname "$0")/../lib.sh"

# module → previously shipped digest (the ONLY value that may read as "stale")
PREV_wave="f4ad27cec4417e2ed5b566adb7f7ee200b3c3fbfddf25adf840d267fd57e417b"
PREV_lexicon="ce9f20f4dc43b79b704a1266ca69956890b006564824c051d44eb31cc90b0329"
PREV_recon="b2b11f1b76e22f7bfac10be3eba7b5104ff0d5d0a1d92147ec3dc392f1475d7d"

stale=""
for m in wave lexicon recon; do
  eval "WANT=\$OCIGER_$(echo "$m" | tr '[:lower:]' '[:upper:]')_SHA256"
  eval "PREV=\$PREV_$m"
  [ -n "$WANT" ] || BROKEN "versions.yaml has no modules: pin for $m — nothing to consult"
  GOT="$(INIMG "/bin/busybox sha256sum /ontology/v3.11/modules/$m.ttl 2>/dev/null" | awk '{print $1}')"
  if [ -z "$GOT" ]; then stale="$stale $m(absent)"; continue; fi
  [ "$GOT" = "$WANT" ] && continue
  if [ "$GOT" = "$PREV" ]; then stale="$stale $m"; continue; fi
  BROKEN "$m.ttl digest $GOT is neither the pinned $WANT nor the known previous $PREV — unvouched bytes"
done

[ -n "$stale" ] && RED "v3.11 modules at previously-shipped bytes:$stale — image predates the re-cut"
GREEN "all v3.11 module files hash to their versions.yaml pins"
