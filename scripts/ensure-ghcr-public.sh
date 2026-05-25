#!/usr/bin/env bash
set -euo pipefail

PACKAGE_NAME="${1:?package name required}"
OWNER="${2:-${GITHUB_REPOSITORY_OWNER:-}}"

if [[ -z "$OWNER" ]]; then
  echo "owner is required" >&2
  exit 1
fi

patch_visibility() {
  local accept="$1"

  gh api \
    --method PUT \
    -H "Accept: ${accept}" \
    -H "X-GitHub-Api-Version: 2026-03-10" \
    "/orgs/${OWNER}/packages/container/${PACKAGE_NAME}/visibility" \
    -f visibility=public
}

if ! patch_visibility "application/vnd.github+json"; then
  patch_visibility "application/vnd.github.package-deletes-preview+json"
fi

gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "/orgs/${OWNER}/packages/container/${PACKAGE_NAME}"
