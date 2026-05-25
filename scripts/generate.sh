#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

while IFS= read -r bundle; do
  go run ./cmd/ociger-gen --bundle "$bundle"
done < <(find bundles -mindepth 2 -maxdepth 2 -name bundle.yaml | sort)
