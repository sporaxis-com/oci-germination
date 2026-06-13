#!/usr/bin/env bash
# cut-plan.sh — read versions.yaml + the bundle FROM graph and print the
# base-first order to cut a component roll-forward wave. The workflow is:
#   1. edit versions.yaml (the single source of truth),
#   2. run this to see WHICH bundles carry the changed components and in WHAT
#      order to cut them (a base must publish before any bundle that FROMs it),
#   3. cut the per-bundle tags top-to-bottom.
# Read-only: no builds, no tags, no edits. Mirrors check-versions.sh parsing
# (portable BSD/macOS grep + sed/awk, no yaml lib). Frozen bundles are listed
# but flagged — they are deliberately off the wave (see versions.yaml).
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

val() { grep -E "^[[:space:]]*$1:[[:space:]]" versions.yaml | head -1 | sed -E 's/^[^"]*"([^"]+)".*/\1/'; }
WANT_PGRDF="$(val pgrdf)"; WANT_PGCK="$(val pgck)"; WANT_CKLIB="$(val cklib)"
frozen_list=$(awk '/^frozen:/{f=1;next} f&&/^[a-zA-Z]/{f=0} f&&/^  [a-zA-Z]/{gsub(/:.*/,"");gsub(/ /,"");print}' versions.yaml)
is_frozen() { printf '%s\n' "$frozen_list" | grep -qx "$1"; }

# Per bundle: its in-repo base dependency (the ociger-* image it FROMs, if any),
# the canonical components it carries, and its release tag pattern.
declare -A DEP COMPS
names=()
for df in bundles/*/Dockerfile; do
  name=$(basename "$(dirname "$df")" | sed 's/^bundle-//')
  names+=("$name")
  DEP[$name]=$(grep -oE 'FROM ghcr\.io/sporaxis-com/ociger-[a-z0-9-]+:' "$df" | head -1 | sed -E 's#.*/ociger-([a-z0-9-]+):#\1#')
  c=""
  grep -q 'pgrdf-bundle:' "$df" && c="$c pgrdf=$WANT_PGRDF"
  grep -q 'styk-tv/pgck:'  "$df" && c="$c pgck=$WANT_PGCK"
  grep -q 'ck-lib-js:'     "$df" && c="$c cklib=$WANT_CKLIB"
  COMPS[$name]="${c# }"
done

# A bundle with a dedicated <name>-release.yml is tagged <name>-v* ; the rest
# (ck-allinone, static-cklib, …) are cut through build-bundles.yml as release-<name>-*.
tagpat() { if [ -f ".github/workflows/$1-release.yml" ]; then echo "$1-v<next>"; else echo "release-$1-v<next>"; fi; }

# Kahn topo-sort over the in-repo FROM edges: emit a bundle once its base is emitted.
emitted=" "; order=(); remaining=("${names[@]}")
while [ ${#remaining[@]} -gt 0 ]; do
  progress=0; next=()
  for n in "${remaining[@]}"; do
    d="${DEP[$n]}"
    if [ -z "$d" ] || printf '%s' "$emitted" | grep -q " $d "; then
      order+=("$n"); emitted="$emitted$n "; progress=1
    else
      next+=("$n")
    fi
  done
  remaining=("${next[@]}")
  if [ $progress -eq 0 ]; then echo "WARN: unresolved base(s) among: ${remaining[*]}" >&2; order+=("${remaining[@]}"); break; fi
done

echo "── cut plan · versions.yaml: pgrdf=$WANT_PGRDF pgck=$WANT_PGCK cklib=$WANT_CKLIB ──"
echo "Cut top-to-bottom; a base publishes before any bundle that FROMs it."
printf '%-4s %-30s %-26s %-30s %s\n' "ord" "bundle" "FROM in-repo base" "components" "release tag"
printf '%-4s %-30s %-26s %-30s %s\n' "───" "──────" "─────────────────" "──────────" "───────────"
i=0
for n in "${order[@]}"; do
  i=$((i+1)); d="${DEP[$n]:-—}"; [ -z "$d" ] && d="—"
  flag=""; is_frozen "$n" && flag="  [FROZEN — not in wave]"
  printf '%-4s %-30s %-26s %-30s %s%s\n' "$i." "$n" "$d" "${COMPS[$n]:-—}" "$(tagpat "$n")" "$flag"
done
