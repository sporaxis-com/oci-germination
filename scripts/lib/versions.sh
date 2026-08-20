#!/usr/bin/env bash
# versions.sh — export the canonical component versions from versions.yaml,
# the SINGLE SOURCE OF TRUTH, so smoke scripts assert against it instead of
# hardcoding their own pins (which drift). Source this, then default each
# EXPECTED_* from the matching OCIGER_* export (env overrides still win):
#
#   source "$(dirname "${BASH_SOURCE[0]}")/lib/versions.sh"
#   EXPECTED_PGRDF_VERSION="${PGRDF_EXPECTED_VERSION:-$OCIGER_PGRDF_VERSION}"
#
# Mirrors check-versions.sh's val() parser (portable BSD/macOS grep + sed, no
# yaml lib). FROZEN bundles (e.g. pg17-pgrdf-pgck-static-cklib) deliberately
# pin OLD versions and MUST NOT source this — they keep their own literals.
_ver_file="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/versions.yaml"
_ver_val() {
  grep -E "^[[:space:]]*$1:[[:space:]]" "$_ver_file" | head -1 | sed -E 's/^[^"]*"([^"]+)".*/\1/'
}
OCIGER_PGRDF_VERSION="$(_ver_val pgrdf)"
OCIGER_PGCK_VERSION="$(_ver_val pgck)"
OCIGER_PGCK_NATIVE="$(_ver_val pgck_native)"
OCIGER_CKLIB_VERSION="$(_ver_val cklib)"
OCIGER_NATS_VERSION="$(_ver_val nats)"
# Ontology module byte-digests (versions.yaml `modules:`) — what the pgck
# artifact ships at /ontology/v3.11/, what init.sql's Adoptions seal, and what
# the ck-allinone smoke pin-ledger gate compares against the sealed rows.
OCIGER_WAVE_SHA256="$(_ver_val wave_sha256)"
OCIGER_LEXICON_SHA256="$(_ver_val lexicon_sha256)"
OCIGER_CORE_SHA256="$(_ver_val core_sha256)"
export OCIGER_PGRDF_VERSION OCIGER_PGCK_VERSION OCIGER_PGCK_NATIVE OCIGER_CKLIB_VERSION OCIGER_NATS_VERSION OCIGER_WAVE_SHA256 OCIGER_LEXICON_SHA256 OCIGER_CORE_SHA256
