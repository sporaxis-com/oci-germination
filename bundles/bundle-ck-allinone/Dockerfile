# syntax=docker/dockerfile:1.7

# ck-allinone Delta — s6-overlay + busybox httpd, scratch base, no Python.
# Marketplace-minimal. Bootstrap of pg extensions is performed by piping
# init.sql through `postgres --single` from inside ociger-pg-launcher,
# so the final image carries no postgres client (psql) and no client libs.

ARG PG_BASE_VERSION=v0.1.16
ARG S6_OVERLAY_VERSION=3.2.3.0
ARG CKLIB_VERSION=1.5.3
# v0.7.11 carries the rewritten ociger-pgck-relay (real NATS→dispatch
# bridge calling ckp.dispatch over TCP+scram as ck_participant); the
# bridge itself is unchanged in v0.7.12+ — only the upstream pg_base
# pin moves with pgRDF/pgCK release cadence.
ARG OCIGER_RELAY_GENERATION=v0.7.11
# Build-arg supplied by CI from the git tag. Default `dev` means a local
# `docker build` without --build-arg lands a clearly-distinguished image.
ARG BUNDLE_VERSION=dev

# ─── Stage 1: pg base (pg17 + pgRDF 0.6.19 + pgCK 0.4.17 + NATS + ociger-pg-launcher) ───
# v0.1.16 carries pgCK v0.4.17 "install-from-zero" — durable tables are
# top-level install DDL born owned by ck_substrate, ontology modules ship
# WITH the artifact at /ontology, and the floor pass covers everything —
# plus pgRDF v0.6.19 (out-of-the-box at-scale staged ingest, T1–T5 GUC levers;
# carve_graph subgraph slicing). pgRDF 0.6.6→0.6.19 is .so/GUC-only — no schema
# change, no base/SONAME move (still 17-bookworm). CREATE EXTENSION pgck CASCADE
# is genuinely sufficient. pgCK 0.4.17 now reports pgck_version() correctly
# (CARGO_PKG_VERSION-derived; the frozen "0.4.3 (rc3)" literal through 0.4.14 is
# fixed) — the smoke asserts the true version.
FROM ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.16 AS pg_base

# ─── Stage 2: cklib browser library (Shape A static layer) ────────────────
# 1.5.3 — the dispatch-only client, verified TYPED-SEALED live against the
# v0.7.17 convergence cut (the consumer half of the three-repo v3.9 loop).
# Ships EXACTLY: LICENSE, README.md, ck.js (the L2 dispatch facade —
# CK/ConceptKernel/ckOn, the instance.*/kernel.* op→verb table, the
# v1.5.3 entry point), ck-client.js (vendored transport, grammar-agnostic
# result.kernel.<K>.> subscription), ck-store.js (the typed-instance cache),
# vendor/{msgpack,nats.ws}.js. No index.html, no ck-page (the legacy
# console is gone). The smoke ⑤± gate asserts this exact file set.
FROM ghcr.io/conceptkernel/ck-lib-js:1.5.3 AS cklib_source

# ─── Stage 3: busybox (provides /bin/busybox; httpd applet inside) ────────
FROM busybox:1.36.1-musl AS busybox_src

# ─── Stage 4: rebuild ociger-pg-launcher fresh ──────────────────────────
# Adds OCIGER_POSTGRES_CONF_EXTRA + OCIGER_INITDB_SQL_FILE. The latter
# is what eliminates psql from the runtime image: postgres single-user
# mode runs the bootstrap SQL after initdb, before the supervised server.
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS pglauncher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
COPY internal/launcher ./internal/launcher
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

# ─── Stage 5: build ociger-pgck-relay ────────────────────────────────────
# Bus-level shim: subscribes input.kernel.pgCK.action.> → re-PUBs to
# event.kernel.pgCK.<verb>. Self-disables when upstream pgck.so embeds
# the relay strings (boot-time probe).
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS pgckrelay_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ociger-pgck-relay ./cmd/ociger-pgck-relay
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w' -o /out/ociger-pgck-relay ./cmd/ociger-pgck-relay

# ─── Stage 6: extract s6-overlay release tarballs into /s6root ────────────
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
ARG BUNDLE_VERSION
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination"
LABEL org.opencontainers.image.description="CKP v3.8 all-in-one minimal — PG17 + pgRDF + pgCK + NATS + cklib, supervised by s6-overlay, web served by busybox httpd. Scratch base, no Python, no postgres client."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.version="${BUNDLE_VERSION}"
LABEL ck.bundle.role="prod"
LABEL ck.bundle.never-prod="false"

# s6-overlay — supervisor + init for the container's services
COPY --from=s6_extract /s6root/ /

# busybox — single static binary, provides httpd applet
COPY --from=busybox_src /bin/busybox /bin/busybox

# cklib — browser JavaScript library mounted at /app/cklib/
COPY --from=cklib_source / /app/cklib/

# Landing page: opens NATS over WSS, does a round-trip, renders ✓/✗ into the DOM.
# The page auto-detects topology (https → wss://host/wss, http :8000 → ws://host:9222).
COPY bundles/bundle-ck-allinone/web/index.html /app/index.html

# Reserved surfaces. /web/ and /web2/ stage the web1 and web2 demo paths so
# downstream consumers can rely on them being served from busybox httpd. /wss/
# is a static documentation page that explains the routing contract — busybox
# does NOT proxy WebSockets; gateways MUST route /wss → :9222 internally.
COPY bundles/bundle-ck-allinone/web/web/index.html  /app/web/index.html
COPY bundles/bundle-ck-allinone/web/web2/index.html /app/web2/index.html
COPY bundles/bundle-ck-allinone/web/wss/index.html  /app/wss/index.html

# Fresh launcher (OCIGER_POSTGRES_CONF_EXTRA + OCIGER_INITDB_SQL_FILE).
COPY --from=pglauncher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher

# Bootstrap SQL — piped through `postgres --single` by the launcher on first boot.
COPY bundles/bundle-ck-allinone/init.sql /etc/ck-allinone/init.sql

# /ontology now arrives from pg_base (pgCK v0.4.2+ ships its ontology modules
# with the extension artifact — canonical source). The vendor copy this bundle
# carried for v0.7.14..v0.7.16 is retired.

# Bus-level dispatcher shim (CHECKLIST §B4). Self-disables when upstream
# pgck.so embeds the relay strings; opt-out via OCIGER_DISABLE_PGCK_RELAY=1.
COPY --from=pgckrelay_build /out/ociger-pgck-relay /usr/local/bin/ociger-pgck-relay

# s6 service definitions (postgres, nats, httpd, pgck-relay) compiled by s6-rc at boot.
COPY bundles/bundle-ck-allinone/s6-services/ /etc/s6-overlay/s6-rc.d/

ENV PGDATA=/var/lib/postgresql/data
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck
ENV OCIGER_POSTGRES_CONF_EXTRA="pgck.nats_url='nats://127.0.0.1:4222'"
ENV OCIGER_INITDB_SQL_FILE=/etc/ck-allinone/init.sql
# v3.9 Critical Isolation Alpha — opt the bundle into the role floor:
# the launcher appends `host all ck_participant 0.0.0.0/0 scram-sha-256`
# to pg_hba.conf on first initdb. Deploys MUST set
# OCIGER_CK_PARTICIPANT_PASSWORD before external exposure; without it,
# ck_participant exists but cannot log in (no default password — a default
# would make the role floor cosmetic per pgCK v3.9 §7).
ENV OCIGER_CK_PARTICIPANT_HBA=1
ENV S6_KEEP_ENV=1
ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2
ENV S6_VERBOSITY=1

EXPOSE 5432 8000 4222 9222
VOLUME ["/var/lib/postgresql/data"]

ENTRYPOINT ["/init"]
