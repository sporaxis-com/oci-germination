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
cd "$(dirname "$0")/.." || exit 1

REPORT_ONLY=0
[ "${1:-}" = "--report" ] && REPORT_ONLY=1

# Minimal YAML reads (no yaml lib): pull the first quoted value off the line
# whose key matches. Portable on BSD/macOS grep + sed (no \s).
val() { grep -E "^[[:space:]]*$1:[[:space:]]" versions.yaml | head -1 | sed -E 's/^[^"]*"([^"]+)".*/\1/'; }

# Read a nested `version:` out of a bundle.yaml — the block key ($2) sits alone
# on its line (`  pgck:`) with `version:` a line or two under it.
#
# Why this exists: until 2026-08-16 this gate read ONLY Dockerfiles, so every
# pin in bundle.yaml was unchecked by anything. bundle-pg18-pgrdf-pgck-nats-micro
# declared pg_major 17, postgres:17-bookworm, pgrdf 0.6.19 and pgck 0.4.20 for a
# pg18/trixie build shipping 0.6.31/0.4.71 — and this script printed `ok`. A pin
# that nothing compares is decoration; comparing it is what makes it a pin.
yaml_ver() {
  grep -A4 -E "^[[:space:]]+$2:[[:space:]]*$" "$1" 2>/dev/null \
    | grep -m1 -E '^[[:space:]]*version:' \
    | sed -E 's/.*version:[[:space:]]*"?([^"#]+)"?.*/\1/' | tr -d ' '
}

WANT_PGRDF="$(val pgrdf)"
WANT_PGCK="$(val pgck)"
WANT_CKLIB="$(val cklib)"

# Frozen bundles (skip drift-failure): everything indented under `frozen:`.
frozen_list=$(awk '/^frozen:/{f=1;next} f&&/^[a-zA-Z]/{f=0} f&&/^  [a-zA-Z]/{gsub(/:.*/,"");gsub(/ /,"");print}' versions.yaml)
is_frozen() { echo "$frozen_list" | grep -qx "$1"; }

echo "── canonical (versions.yaml): pgrdf=$WANT_PGRDF pgck=$WANT_PGCK cklib=$WANT_CKLIB ──"
printf '%-40s %-12s %-12s %-10s %-12s %s\n' "bundle" "pgrdf" "pgck" "cklib" "bundle.yaml" "status"
printf '%-40s %-12s %-12s %-10s %-12s %s\n' "──────" "─────" "────" "─────" "───────────" "──────"

drift=0
details=""
# EVERY bundle dir carrying a Dockerfile — not just `bundle-*`. The old glob
# silently skipped core-pg17{,-micro,-nats,-nats-micro}: four bundles that are
# in scripts/cut-plan.sh's wave and absent from `frozen:`, so they were neither
# checked nor excused. A gate whose coverage is a filename prefix is one rename
# away from lying.
for df in bundles/*/Dockerfile; do
  [ -f "$df" ] || continue
  dir=$(dirname "$df")
  bundle=$(basename "$dir" | sed 's/^bundle-//')
  got_pgrdf=$(grep -oE 'pgrdf-bundle:[0-9.]+' "$df" | head -1 | cut -d: -f2)
  got_pgck=$(grep -oE 'pgck:[0-9.]+' "$df" | head -1 | cut -d: -f2)
  got_cklib=$(grep -oE 'ck-lib-js:[0-9.]+' "$df" | head -1 | cut -d: -f2)

  # The declared half — what bundle.yaml SAYS this bundle contains.
  yml="$dir/bundle.yaml"
  ymismatch=0
  ystatus="—"
  if [ -f "$yml" ]; then
    y_pgrdf=$(yaml_ver "$yml" pgrdf)
    y_pgck=$(yaml_ver "$yml" pgck)
    y_cklib=$(yaml_ver "$yml" cklib)
    for pair in "pgrdf:$y_pgrdf:$WANT_PGRDF" "pgck:$y_pgck:$WANT_PGCK" "cklib:$y_cklib:$WANT_CKLIB"; do
      n="${pair%%:*}"; rest="${pair#*:}"; got="${rest%%:*}"; want="${rest##*:}"
      if [ -n "$got" ] && [ "$got" != "$want" ]; then
        ymismatch=1
        details="${details}    ${bundle}: bundle.yaml declares ${n} ${got}, versions.yaml says ${want}\n"
      fi
    done
    [ "$ymismatch" = 1 ] && ystatus="DRIFT" || ystatus="ok"
  fi

  status="ok"
  mismatch=0
  [ -n "$got_pgrdf" ] && [ "$got_pgrdf" != "$WANT_PGRDF" ] && mismatch=1
  [ -n "$got_pgck" ]  && [ "$got_pgck"  != "$WANT_PGCK"  ] && mismatch=1
  [ -n "$got_cklib" ] && [ "$got_cklib" != "$WANT_CKLIB" ] && mismatch=1
  [ "$ymismatch" = 1 ] && mismatch=1

  if is_frozen "$bundle"; then
    status="frozen"
  elif [ "$mismatch" = 1 ]; then
    status="DRIFT"
    drift=$((drift+1))
  fi
  printf '%-40s %-12s %-12s %-10s %-12s %s\n' \
    "$bundle" "${got_pgrdf:-—}" "${got_pgck:-—}" "${got_cklib:-—}" "$ystatus" "$status"
done

if [ -n "$details" ]; then
  echo ""
  echo "  declared-vs-canonical mismatches (bundle.yaml):"
  printf "%b" "$details"
fi

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
