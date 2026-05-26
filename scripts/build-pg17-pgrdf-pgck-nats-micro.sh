#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

platform="${OCI_GER_PLATFORM:-}"
if [[ -z "$platform" ]]; then
  case "$(docker info --format '{{.Architecture}}')" in
    aarch64|arm64)
      platform="linux/arm64"
      ;;
    x86_64|amd64)
      platform="linux/amd64"
      ;;
    *)
      echo "unsupported docker architecture; set OCI_GER_PLATFORM explicitly" >&2
      exit 1
      ;;
  esac
fi

bash scripts/generate.sh
docker buildx build \
  --load \
  --platform "$platform" \
  -f bundles/bundle-pg17-pgrdf-pgck-nats-micro/Dockerfile \
  -t ociger-pg17-pgrdf-pgck-nats-micro:local \
  .
