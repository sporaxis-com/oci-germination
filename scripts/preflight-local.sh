#!/usr/bin/env bash
# preflight-local.sh — the whole local gate, one command, BEFORE any tag.
#
#   bash scripts/preflight-local.sh          # full loop: build + smoke + tdd
#   bash scripts/preflight-local.sh --no-build   # gates that need no image build
#
# Codifies the sequence every release of this repo has actually needed, in the
# order failures are cheapest: static gates first, then the base image, then
# the bundle, then the suites that boot containers. Every stage's exit code is
# consulted directly — never through a pipe (a piped gate reports the pipe's
# status; measured twice on this bench, both times agreeing with the truth by
# accident).
#
# The three-state suite runs LAST, against the image this loop just built, so
# a preflight pass means: drift clean, go tests green, both images built and
# smoked, identity path proven under a bind mount, and every remaining RED in
# tests/local-tdd is a *predicted* gap, not a surprise.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NOBUILD=""
[ "${1:-}" = "--no-build" ] && NOBUILD=1

step() { echo; echo "════ preflight: $1"; }

step "versions drift gate"
bash scripts/check-versions.sh

step "gofmt (must emit nothing)"
FMT="$(gofmt -l cmd/ internal/)"
[ -z "$FMT" ] || { echo "gofmt needed:"; echo "$FMT"; exit 1; }

step "go test ./..."
go test ./...

step "pgRDF preload lint"
bash scripts/lint-pgrdf-preload.sh

step "generator adds no drift (CI parity, dirty-tree safe)"
# CI runs `generate && git diff --exit-code` on a CLEAN checkout. Locally the
# tree is often legitimately dirty (staged pins awaiting a coordinated cut), so
# assert the generator CHANGES nothing — not that nothing is changed.
BEFORE="$(git diff -- bundles/ | shasum -a 256 | awk '{print $1}')"
bash scripts/generate.sh >/dev/null
AFTER="$(git diff -- bundles/ | shasum -a 256 | awk '{print $1}')"
[ "$BEFORE" = "$AFTER" ] || { echo "generate.sh modified bundles/ — regenerate and commit before cutting"; git diff --stat -- bundles/; exit 1; }

if [ -n "$NOBUILD" ]; then
  echo; echo "preflight (--no-build): static gates PASS. Build stages skipped."
  exit 0
fi

step "build base micro"
bash scripts/build-pg18-pgrdf-pgck-nats-micro.sh

step "smoke base micro"
bash scripts/smoke-pg18-pgrdf-pgck-nats-micro.sh

step "retag base for the bundle FROM (local tag only — never pushed by this script)"
BASE_VER="$(grep -E '^  pg18-pgrdf-pgck-nats-micro:' versions.yaml | sed -E 's/^[^"]*"([^"]+)".*/\1/')"
docker tag ociger-pg18-pgrdf-pgck-nats-micro:local \
  "ghcr.io/sporaxis-com/ociger-pg18-pgrdf-pgck-nats-micro:${BASE_VER}"

step "build ck-allinone (candidate)"
docker build -f bundles/bundle-ck-allinone/Dockerfile \
  --build-arg BUNDLE_VERSION=local-preflight \
  --build-arg GIT_SHA="$(git rev-parse --short HEAD)" \
  -t ociger-ck-allinone:local-preflight .

step "smoke ck-allinone"
bash scripts/smoke-ck-allinone.sh ociger-ck-allinone:local-preflight

step "verify-callout (identity under a bind mount)"
bash scripts/verify-callout.sh ociger-ck-allinone:local-preflight

step "three-state suite against the candidate"
OG_TDD_IMAGE=ociger-ck-allinone:local-preflight bash tests/local-tdd/run.sh

echo
echo "════ preflight PASS — candidate ociger-ck-allinone:local-preflight is tag-ready"
echo "     (cutting is still a separate, explicit act: push commits, then one tag at a time)"
