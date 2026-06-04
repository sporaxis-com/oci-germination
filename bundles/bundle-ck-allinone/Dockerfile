# syntax=docker/dockerfile:1.7

# ck-allinone Delta — s6-overlay + busybox httpd, scratch base, no Python.
# Marketplace-minimal: zero bespoke binaries beyond what pg_base already
# carries (ociger-pg-launcher). Supervisor = s6 (audited, well-known).
# Web layer = busybox httpd (static, ~1 MB, no config language needed).
# All extension binaries (pgrdf.so, pgck.so) and the postgres server
# inherit from pg_base, which is pgRDF's de-facto compile target
# (Debian bookworm SONAMEs preserved end-to-end).

ARG PG_BASE_VERSION=v0.1.7
ARG S6_OVERLAY_VERSION=3.2.3.0
ARG CKLIB_VERSION=1.3.11

# ─── Stage 1: pg base (pg17 + pgRDF + pgCK + NATS + ociger-pg-launcher) ───
FROM ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.7 AS pg_base

# ─── Stage 2: cklib browser library (Shape A static layer) ────────────────
FROM ghcr.io/conceptkernel/ck-lib-js:1.3.11 AS cklib_source

# ─── Stage 3: busybox (provides /bin/busybox; httpd applet inside) ────────
FROM busybox:1.36.1-musl AS busybox_src

# ─── Stage 3b: extract psql + its NEEDED libs from bookworm ───────────────
# Used by the §A bootstrap oneshot to run CREATE EXTENSION on first boot.
# /usr/bin/psql in bookworm is the pg_wrapper Perl script — the real ELF
# binary lives at /usr/lib/postgresql/17/bin/psql.
FROM postgres:17-bookworm AS psql_src
RUN set -eux; \
    mkdir -p /out/usr/local/bin; \
    cp /usr/lib/postgresql/17/bin/psql /out/usr/local/bin/psql; \
    ldd /usr/lib/postgresql/17/bin/psql \
      | tr -s '[:space:]' '\n' | grep '^/' | sort -u \
      | xargs -r -I '{}' cp --parents '{}' /out

# ─── Stage 3c: rebuild ociger-pg-launcher fresh ──────────────────────────
# pg_base ships an older revision of ociger-pg-launcher that doesn't know
# about OCIGER_POSTGRES_CONF_EXTRA. Needed for §A to bake pgck.nats_url
# into postgresql.conf on first initdb (pgCK's bgworker reads the GUC once
# on first tick — SIGHUP after-the-fact does not wake it).
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS pglauncher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
COPY internal/launcher ./internal/launcher
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

# ─── Stage 3d: build ociger-pgck-relay ───────────────────────────────────
# Bus-level shim that subscribes input.kernel.pgCK.action.> and re-emits
# event.kernel.pgCK.<verb>. Stands in for the relay code that the current
# published pgck.so was compiled without. Self-disables (boot probe) the
# moment an upstream pgck.so ships with the relay strings embedded.
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS pgckrelay_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ociger-pgck-relay ./cmd/ociger-pgck-relay
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w' -o /out/ociger-pgck-relay ./cmd/ociger-pgck-relay

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
LABEL ck.bundle.role="prod"
LABEL ck.bundle.never-prod="false"

# s6-overlay — supervisor + init for the container's services
COPY --from=s6_extract /s6root/ /

# busybox — single static binary, provides httpd applet
COPY --from=busybox_src /bin/busybox /bin/busybox

# cklib — browser JavaScript library mounted at /app/cklib/
# busybox httpd will serve /app on :8000, so cklib is at /cklib/ over HTTP
COPY --from=cklib_source / /app/cklib/

# Landing page: opens NATS over WSS to ws://${host}:9222, does a round-trip
# subscribe→publish→receive, renders ✓/✗ into the DOM. busybox httpd serves
# /app/index.html for /  by default.
COPY bundles/bundle-ck-allinone/web/index.html /app/index.html

# §A auto-bootstrap: fresh launcher (knows OCIGER_POSTGRES_CONF_EXTRA),
# psql + libs, bootstrap-ckp.sh, and the s6 oneshot.
COPY --from=pglauncher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
COPY --from=psql_src /out/ /
COPY bundles/bundle-ck-allinone/scripts/bootstrap-ckp.sh /usr/local/bin/bootstrap-ckp.sh
RUN chmod +x /usr/local/bin/bootstrap-ckp.sh || true

# §B dispatcher: bus-level relay (input.kernel.pgCK.action.> → event.kernel.pgCK.<verb>).
# Self-disables when upstream pgck.so embeds the relay strings; opt-out via
# OCIGER_DISABLE_PGCK_RELAY=1 if a future image lands the upstream build before
# the probe catches it.
COPY --from=pgckrelay_build /out/ociger-pgck-relay /usr/local/bin/ociger-pgck-relay

# s6 service definitions (postgres, nats, httpd) compiled by s6-rc at boot
COPY bundles/bundle-ck-allinone/s6-services/ /etc/s6-overlay/s6-rc.d/

ENV PGDATA=/var/lib/postgresql/data
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck
# §A auto-bootstrap: pgck.nats_url baked into postgresql.conf on first
# initdb so the pgCK bgworker reads it on its single get_or_init tick.
ENV OCIGER_POSTGRES_CONF_EXTRA="pgck.nats_url='nats://127.0.0.1:4222'"
ENV S6_KEEP_ENV=1
ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2
ENV S6_VERBOSITY=1

EXPOSE 5432 8000 4222 9222
VOLUME ["/var/lib/postgresql/data"]

ENTRYPOINT ["/init"]
