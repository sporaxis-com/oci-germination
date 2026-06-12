#!/usr/bin/env bash
# check-versions.sh — drift gate against versions.yaml (the single source of truth).
#
# Scans every bundle Dockerfile for its pgRDF / pgCK / cklib pin and reports
# whether it matches the canonical version in versions.yaml. Exits non-zero if
# any non-frozen bundle drifts — wire it into CI so the matrix can never
# silently rot again.
#
# Usage:  scripts/check-versions.sh            # report + gate
#         scripts/check-versions.sh --report   # report only, always exit 0
set -uo pipefail
cd "$(dirname "$0")/.."

REPORT_ONLY=0
[ "${1:-}" = "--report" ] && REPORT_ONLY=1

# Minimal YAML reads (no yaml lib): pull the first quoted value off the line
# whose key matches. Portable on BSD/macOS grep + sed (no \s).
val() { grep -E "^[[:space:]]*$1:[[:space:]]" versions.yaml | head -1 | sed -E 's/^[^"]*"([^"]+)".*/\1/'; }

WANT_PGRDF="$(val pgrdf)"
WANT_PGCK="$(val pgck)"
WANT_CKLIB="$(val cklib)"

# Frozen bundles (skip drift-failure): everything indented under `frozen:`.
frozen_list=$(awk '/^frozen:/{f=1;next} f&&/^[a-zA-Z]/{f=0} f&&/^  [a-zA-Z]/{gsub(/:.*/,"");gsub(/ /,"");print}' versions.yaml)
is_frozen() { echo "$frozen_list" | grep -qx "$1"; }

echo "── canonical (versions.yaml): pgrdf=$WANT_PGRDF pgck=$WANT_PGCK cklib=$WANT_CKLIB ──"
printf '%-40s %-12s %-12s %-10s %s\n' "bundle" "pgrdf" "pgck" "cklib" "status"
printf '%-40s %-12s %-12s %-10s %s\n' "──────" "─────" "────" "─────" "──────"

drift=0
for df in bundles/bundle-*/Dockerfile; do
  bundle=$(basename "$(dirname "$df")" | sed 's/^bundle-//')
  got_pgrdf=$(grep -oE 'pgrdf-bundle:[0-9.]+' "$df" | head -1 | cut -d: -f2)
  got_pgck=$(grep -oE 'pgck:[0-9.]+' "$df" | head -1 | cut -d: -f2)
  got_cklib=$(grep -oE 'ck-lib-js:[0-9.]+' "$df" | head -1 | cut -d: -f2)

  status="ok"
  mismatch=0
  [ -n "$got_pgrdf" ] && [ "$got_pgrdf" != "$WANT_PGRDF" ] && mismatch=1
  [ -n "$got_pgck" ]  && [ "$got_pgck"  != "$WANT_PGCK"  ] && mismatch=1
  [ -n "$got_cklib" ] && [ "$got_cklib" != "$WANT_CKLIB" ] && mismatch=1

  if is_frozen "$bundle"; then
    status="frozen"
  elif [ "$mismatch" = 1 ]; then
    status="DRIFT"
    drift=$((drift+1))
  fi
  printf '%-40s %-12s %-12s %-10s %s\n' \
    "$bundle" "${got_pgrdf:-—}" "${got_pgck:-—}" "${got_cklib:-—}" "$status"
done

# Bundles that FROM a derived base (ck-allinone, static-cklib) carry their
# components transitively via the base pin, not a direct pgrdf-bundle:/pgck:
# line — shown as "—" above; their components are whatever the base ships.
echo ""
if [ "$drift" -gt 0 ]; then
  echo "✗ $drift bundle(s) drift from versions.yaml. Bring them to the canonical pins (or add to frozen: with a reason)."
  [ "$REPORT_ONLY" = 1 ] && exit 0
  exit 1
fi
echo "✓ all non-frozen bundles match versions.yaml"
