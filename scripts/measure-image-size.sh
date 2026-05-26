#!/usr/bin/env bash
set -euo pipefail

IMAGE="$1"
SLUG="$2"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$ROOT/.artifacts"

UNCOMPRESSED="$(docker image inspect "$IMAGE" --format '{{.Size}}')"
ARCHIVE="$ROOT/.artifacts/${SLUG}.tar.gz"
docker save "$IMAGE" | gzip -c > "$ARCHIVE"
COMPRESSED="$(wc -c < "$ARCHIVE")"

echo "uncompressed_bytes=$UNCOMPRESSED"
echo "compressed_bytes=$COMPRESSED"
