# syntax=docker/dockerfile:1.7

# ck-allinone Delta — s6-overlay + busybox httpd, scratch base, no Python.
# Marketplace-minimal: zero bespoke binaries beyond what pg_base already
# carries (ociger-pg-launcher). Supervisor = s6 (audited, well-known).
# Web layer = busybox httpd (static, ~1 MB, no config language needed).
# All extension binaries (pgrdf.so, pgck.so) and the postgres server
# inherit from pg_base, which is pgRDF's de-facto compile target
# (Debian bookworm SONAMEs preserved end-to-end).

ARG PG_BASE_VERSION=v0.1.5
ARG S6_OVERLAY_VERSION=3.2.3.0
ARG CKLIB_VERSION=1.3.11

# ─── Stage 1: pg base (pg17 + pgRDF + pgCK + NATS + ociger-pg-launcher) ───
FROM ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.5 AS pg_base

# ─── Stage 2: cklib browser library (Shape A static layer) ────────────────
FROM ghcr.io/conceptkernel/ck-lib-js:1.3.11 AS cklib_source

# ─── Stage 3: busybox (provides /bin/busybox; httpd applet inside) ────────
FROM busybox:1.36.1-musl AS busybox_src

# ─── Stage 4: extract s6-overlay release tarballs into /s6root ────────────
FROM debian:trixie-slim AS s6_extract
ARG S6_OVERLAY_VERSION
ARG TARGETARCH
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl xz-utils ca-certificates && \
    rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    case "$TARGETARCH" in \
        amd64) S6_ARCH=x86_64 ;; \
        arm64) S6_ARCH=aarch64 ;; \
        *)     echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    mkdir /s6root; \
    curl -fsSL "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz" \
        | tar -C /s6root -Jxpf -; \
    curl -fsSL "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz" \
        | tar -C /s6root -Jxpf -

# ─── Final stage: layer everything on pg_base ─────────────────────────────
FROM pg_base
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination"
LABEL org.opencontainers.image.description="CKP v3.8 all-in-one minimal — PG17 + pgRDF + pgCK + NATS + cklib, supervised by s6-overlay, web served by busybox httpd. No Python, no FastAPI, scratch base."
LABEL org.opencontainers.image.licenses="MIT"

# s6-overlay — supervisor + init for the container's services
COPY --from=s6_extract /s6root/ /

# busybox — single static binary, provides httpd applet
COPY --from=busybox_src /bin/busybox /bin/busybox

# cklib — browser JavaScript library mounted at /app/cklib/
# busybox httpd will serve /app on :8000, so cklib is at /cklib/ over HTTP
COPY --from=cklib_source / /app/cklib/

# s6 service definitions (postgres, nats, httpd) compiled by s6-rc at boot
COPY bundles/bundle-ck-allinone/s6-services/ /etc/s6-overlay/s6-rc.d/

ENV PGDATA=/var/lib/postgresql/data
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck
ENV S6_KEEP_ENV=1
ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2
ENV S6_VERBOSITY=1

EXPOSE 5432 8000 4222 9222
VOLUME ["/var/lib/postgresql/data"]

ENTRYPOINT ["/init"]
