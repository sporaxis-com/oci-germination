#!/usr/bin/env bash
# scripts/gh-watch.sh <tag>
#
# Watch the GHA chain for a release tag and report when it's "in":
#   1. resolve <tag> → commit SHA (via `git rev-list -n1`)
#   2. pick the primary workflow by tag-pattern
#   3. gh run watch the primary run on that SHA → exit-status
#   4. gh run watch the chained update-latest-md.yml run on the same SHA
#   5. print ✓ + first 20 lines of LATEST.md OR ✗ on any failure
#
# Adapted from /Users/neoxr/git_conceptkernel/pgCK/_WIP/SPEC.CLAUDE.GH-WATCH.v0.2.md
# for oci-germination's workflow graph (Build OCI Bundles + 8 per-bundle
# release.yml + update-latest-md.yml).
#
# Usage:
#   scripts/gh-watch.sh release-ck-allinone-v0.6.4
#   Bash(command="scripts/gh-watch.sh <tag>", run_in_background=true)

set -euo pipefail

REPO="sporaxis-com/oci-germination"

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <tag>" >&2
  exit 2
fi
TAG="$1"

# macOS notification helper (no-op if osascript missing)
notify() {
  local title="$1" body="$2" sound="$3"
  command -v osascript >/dev/null 2>&1 || return 0
  osascript -e "display notification \"${body}\" with title \"${title}\" sound name \"${sound}\"" >/dev/null 2>&1 || true
}

on_fail() {
  local msg="${1:-FAILED}"
  echo "✗ Release ${msg}: ${TAG}" >&2
  notify "oci-germination" "Release ${msg}: ${TAG}" "Sosumi"
  exit 1
}
trap 'on_fail FAILED' ERR

# Tag-pattern → primary workflow name (must match `name:` in the YAML)
pick_workflow() {
  local tag="$1"
  case "$tag" in
    release-ck-allinone-v*|release-pg17-pgrdf-pgck-web-cklib-v*|release-pg17-pgrdf-pgck-static-cklib-v*)
      echo "Build OCI Bundles" ;;
    core-pg17-v*)                      echo "core-pg17-release" ;;
    core-pg17-micro-v*)                echo "core-pg17-micro-release" ;;
    core-pg17-nats-v*)                 echo "core-pg17-nats-release" ;;
    core-pg17-nats-micro-v*)           echo "core-pg17-nats-micro-release" ;;
    pg17-pgrdf-v*)                     echo "pg17-pgrdf-release" ;;
    pg17-pgrdf-pgck-v*)                echo "pg17-pgrdf-pgck-release" ;;
    pg17-pgrdf-pgck-nats-v*)           echo "pg17-pgrdf-pgck-nats-release" ;;
    pg17-pgrdf-pgck-nats-micro-v*)     echo "pg17-pgrdf-pgck-nats-micro-release" ;;
    *) on_fail "unknown tag pattern: ${tag}" ;;
  esac
}

# Resolve the commit SHA the tag points at.
SHA=$(git rev-list -n1 "${TAG}" 2>/dev/null) || on_fail "tag not found locally: ${TAG}"
SHORT_SHA=${SHA:0:8}
PRIMARY=$(pick_workflow "${TAG}")

echo "Watching: tag=${TAG} sha=${SHORT_SHA} primary=\"${PRIMARY}\""

# Find the primary run id for this SHA + workflow.
# Poll up to 60s for the run to appear (GitHub queues for a few seconds after push).
PRIMARY_RUN=""
for _ in $(seq 1 12); do
  PRIMARY_RUN=$(gh run list --repo "${REPO}" --workflow "${PRIMARY}" --commit "${SHA}" --limit 1 --json databaseId --jq '.[0].databaseId // ""')
  [[ -n "${PRIMARY_RUN}" ]] && break
  sleep 5
done
[[ -n "${PRIMARY_RUN}" ]] || on_fail "no ${PRIMARY} run found on ${SHORT_SHA}"

echo "Primary run: ${PRIMARY_RUN} (gh run watch)"
gh run watch --repo "${REPO}" "${PRIMARY_RUN}" --exit-status >/dev/null || on_fail "primary build FAILED (${PRIMARY_RUN})"

# Find the update-latest-md.yml run triggered by the successful primary.
# It's a workflow_run event chained from the primary. Look for runs with the
# same head_sha and workflow name "update-latest-md".
ULM_RUN=""
for _ in $(seq 1 24); do
  ULM_RUN=$(gh run list --repo "${REPO}" --workflow "update-latest-md" --commit "${SHA}" --limit 1 --json databaseId --jq '.[0].databaseId // ""')
  [[ -n "${ULM_RUN}" ]] && break
  sleep 5
done
[[ -n "${ULM_RUN}" ]] || on_fail "update-latest-md run never appeared for ${SHORT_SHA}"

echo "update-latest-md run: ${ULM_RUN} (gh run watch)"
gh run watch --repo "${REPO}" "${ULM_RUN}" --exit-status >/dev/null || on_fail "update-latest-md FAILED (${ULM_RUN})"

# Success path.
echo "✓ Release in: ${TAG}"
notify "oci-germination" "Release in: ${TAG}" "Glass"
echo ""
echo "--- LATEST.md (local; run 'git pull --ff-only origin main' to refresh) ---"
head -20 LATEST.md 2>/dev/null || echo "(LATEST.md not found locally)"
