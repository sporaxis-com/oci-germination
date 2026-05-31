# syntax=docker/dockerfile:1.7

# ociger-pgck-bench — Python+FastAPI sibling for benchmarking and dev workflows
# against a running ck-allinone (or any equivalent attested pg+pgRDF+pgCK+NATS
# target). This is the ONLY sanctioned home for Python in the ociger group.
#
# Architecture (per 2026-05-31 marketplace mandate):
#   ┌───────────────────┐         NATS WSS / pg wire        ┌────────────────────┐
#   │ ociger-pgck-bench │ ──────────────────────────────▶  │ ociger-ck-allinone │
#   │ (Python+FastAPI)  │   (sibling network, not embed)    │ (Delta, no Python) │
#   └───────────────────┘                                   └────────────────────┘
#
# Why sibling, not sidecar/profile:
#   1. ck-allinone is marketplace-minimal (~51 MB compressed) — adding Python
#      would more than double the image and triple the RSS budget.
#   2. Python+pgck-web's base release (Debian trixie) collides at the SONAME
#      level with pg_base's bookworm-compiled binaries — putting them in the
#      same image needs apt-band-aids that don't satisfy the linker anyway.
#   3. Bench/dev tools have a different upgrade cadence than the prod cluster
#      runtime; separating images lets them evolve independently.
#
# This Dockerfile is a thin layer over the upstream pgck-web image so this repo
# does not compile or re-derive anything — consistent with the [[oci-germination
# never compiles]] discipline. Tag explicitly with -bench / -devel to make the
# non-prod nature unmistakable on GHCR.

ARG PGCKWEB_VERSION=v0.2.7

FROM ghcr.io/styk-tv/pgck-web:v0.2.7-${TARGETARCH:-amd64} AS upstream

FROM upstream
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination"
LABEL org.opencontainers.image.description="Python+FastAPI sibling for benchmarking and dev against a running ck-allinone. NOT for production. Connects to external PG (5432) and NATS WSS (9222) on the same network."
LABEL org.opencontainers.image.licenses="MIT"
LABEL ck.bundle.role="bench"
LABEL ck.bundle.never-prod="true"

# Defaults assume a sibling ck-allinone on the same docker network at hostname
# 'ck-allinone'. Override at runtime for other targets.
ENV CK_PG_HOST=ck-allinone
ENV CK_PG_PORT=5432
ENV CK_PG_USER=postgres
ENV CK_PG_DB=postgres
ENV CK_NATS_URL=nats://ck-allinone:4222
ENV CK_NATS_WSS_URL=ws://ck-allinone:9222
ENV CK_BENCH_MODE=1

EXPOSE 8000

# Inherit pgck-web's entrypoint (uvicorn web.app:app on :8000); benchmark
# harnesses talk to this surface from outside the container.
