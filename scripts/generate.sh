#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

go run ./cmd/ociger-gen --bundle bundles/core-pg17/bundle.yaml
