# syntax=docker/dockerfile:1.7

# ck-allinone Delta — s6-overlay + busybox httpd, scratch base, no Python.
# Marketplace-minimal. Bootstrap of pg extensions is performed by piping
# init.sql through `postgres --single` from inside ociger-pg-launcher,
# so the final image carries no postgres client (psql) and no client libs.

ARG PG_BASE_VERSION=v0.2.8
ARG S6_OVERLAY_VERSION=3.2.3.0
ARG CKLIB_VERSION=1.5.12
# Build-arg supplied by CI from the git tag. Default `dev` means a local
# `docker build` without --build-arg lands a clearly-distinguished image.
ARG BUNDLE_VERSION=dev
# v0.7.31: the source commit, stamped into EVERY arch as
# org.opencontainers.image.revision. CI's cross-arch parity gate asserts every
# published arch carries the same revision AND the same runtime glibc, so
# "amd64 and arm64 came from one commit" is enforced, not asserted.
ARG GIT_SHA=unknown

# ─── Stage 1: pg base (pg18/trixie + pgRDF 0.6.33 + pgCK 0.4.81-nats + NATS) ───
# v0.2.5 carries the 0.6.33 / 0.4.77 wave — the pair the two-Adoption init contract
# needs on fresh installs: pgRDF 0.6.33 fixes the refused-Adoption shmem poison
# (refuse→retry now seals with no pgrdf.shmem_reset()), and pgCK 0.4.77 creates
# ckp.adoption_pins at CREATE EXTENSION (previously bootstrap_kernel/migration-only,
# so the SECOND Adoption died mid-recomposition on every fresh install). Both halves
# verified working together on the shipped, attested artifacts 2026-08-20 (pgCK's
# fresh-install gate: boot 27 shapes → two governed Adoptions → fleet.adoptions
# answers → 42 composed). The -nats build still owns the in-extension inbound
# dispatch and the auth-callout, so the Go relay stays deleted.
# Base remains postgres:18-trixie (glibc 2.41; the pgRDF arm64 .so needs GLIBC_2.39).
FROM ghcr.io/sporaxis-com/ociger-pg18-pgrdf-pgck-nats-micro:v0.2.8 AS pg_base

# ─── Stage 2: cklib browser library (Shape A static layer) ────────────────
# 1.5.12 — the version pgck.localhost itself serves from /cklib/, so the browser
# and the door run one library. Adds tokenProvider (#14 Mode A): the app owns the
# bearer and cklib forwards it into the CONNECT frame, parsing claims for display
# but never validating — admission stays the substrate's call.
# Ships: LICENSE, README.md, ck.js (the L2 dispatch facade — CK/ConceptKernel/ckOn,
# the instance.*/kernel.* op→verb table), ck-client.js (vendored transport,
# grammar-agnostic result.kernel.<K>.> subscription), ck-store.js (typed-instance
# cache), vendor/{msgpack,nats.ws}.js. No index.html.
# The smoke ⑤± gate asserts this file set BYTE-FOR-BYTE; re-measured against
# 1.5.12 on 2026-08-20 — identical 7-file set, the 1.5.5-era list still holds.
FROM ghcr.io/conceptkernel/ck-lib-js:1.5.12 AS cklib_source

# ─── Stage 3: busybox (provides /bin/busybox; httpd applet inside) ────────
FROM busybox:1.36.1-musl AS busybox_src

# ─── Stage 4: rebuild ociger-pg-launcher fresh ──────────────────────────
# Adds OCIGER_POSTGRES_CONF_EXTRA + OCIGER_INITDB_SQL_FILE. The latter
# is what eliminates psql from the runtime image: postgres single-user
# mode runs the bootstrap SQL after initdb, before the supervised server.
FROM --platform=$BUILDPLATFORM golang:1.24-trixie AS pglauncher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
COPY internal/launcher ./internal/launcher
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

# ─── (v0.7.30) Go ociger-pgck-relay stage REMOVED ────────────────────────
# pgCK 0.4.24-nats owns the in-extension inbound dispatch (F1-inbound, hop 4):
# it sets ckp.requester from the broker-enforced verified identity before
# ckp.dispatch. The Go relay's inbound bridge is fully superseded — building it
# alongside 0.4.24 would double-dispatch every inbound action — so its build
# stage, binary COPY, s6 service, and cmd/ociger-pgck-relay are all deleted.

# ─── Stage 5: build ociger-ck-identity (boot-time auth-callout provisioner) ──
# Runs as an s6 oneshot BEFORE nats + postgres. Resolves the callout ACCOUNT
# nkey (operator OCIGER_NATS_ACCOUNT_SEED, else minted per-boot), then writes
# /etc/nats/nats-server.conf (auth_callout issuer = account pubkey) + the
# /run/ck-identity/pgck.conf fragment postgres pulls in (pgck.nats_account_seed
# + worker-cred nats_url). The seed is NEVER baked into the image.
FROM --platform=$BUILDPLATFORM golang:1.24-trixie AS ckidentity_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ociger-ck-identity ./cmd/ociger-ck-identity
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w' -o /out/ociger-ck-identity ./cmd/ociger-ck-identity

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
ARG GIT_SHA
LABEL org.opencontainers.image.revision="${GIT_SHA}"
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination"
LABEL org.opencontainers.image.description="CKP v3.11 all-in-one minimal — PG18 (trixie) + pgRDF 0.6.33 + pgCK 0.4.81-nats + NATS + cklib 1.5.12, supervised by s6-overlay, web served by busybox httpd. Scratch base, no Python, no postgres client, no Go relay (pgCK owns in-extension inbound dispatch + auth-callout)."
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

# (v0.7.30) No Go relay COPY — pgCK 0.4.24-nats owns inbound dispatch in-.so.

# Boot-time auth-callout provisioner (the ck-identity s6 oneshot runs it before
# nats + postgres): mints/derives the callout account and writes both
# /etc/nats/nats-server.conf and /run/ck-identity/pgck.conf from one account.
COPY --from=ckidentity_build /out/ociger-ck-identity /usr/local/bin/ociger-ck-identity

# nats-server.conf here is the PRE-ONESHOT FALLBACK only. On every boot the
# ck-identity oneshot OVERWRITES it with the auth_callout config (issuer = the
# resolved account pubkey; anon connections become subscribe-only via the
# callout — strictly stronger than this file's interim forge-deny). Because
# nats depends on ck-identity, nats never actually starts on this fallback; it
# is kept only as a safe default if the oneshot is disabled.
COPY bundles/bundle-ck-allinone/nats-server.conf /etc/nats/nats-server.conf

# s6 service definitions (ck-identity oneshot → postgres, nats, httpd) compiled
# by s6-rc at boot. nats + postgres depend on ck-identity (it writes their config).
COPY bundles/bundle-ck-allinone/s6-services/ /etc/s6-overlay/s6-rc.d/

ENV PGDATA=/var/lib/postgresql/data
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck
# (v0.7.30) postgresql.conf pulls the boot-generated pgck secrets (nats_url with
# worker creds + pgck.nats_account_seed) from the ck-identity oneshot's output
# via include_if_exists — re-read every boot, so a freshly-minted account stays
# consistent with the nats-server.conf issuer. The launcher appends this line at
# first initdb.
ENV OCIGER_POSTGRES_CONF_EXTRA="include_if_exists = '/run/ck-identity/pgck.conf'"
ENV OCIGER_INITDB_SQL_FILE=/etc/ck-allinone/init.sql
# (v0.7.30) OPTIONAL identity-plane env (read by ck-identity at boot; not set here
# — supplied by the deploy). Provide ALL THREE to activate the auth-callout so
# dispatch is identity-gated (verified sub → ckp.requester → sealed `by:`):
#   OCIGER_OIDC_JWKS       the realm's JWKS ({"keys":[{kty:OKP,crv:Ed25519,kid,x}]})
#   OCIGER_OIDC_ISSUER     the realm token issuer (JWT `iss`)
#   OCIGER_OIDC_AUDIENCE   the client audience (JWT `aud`)
# Optional with a realm: OCIGER_NATS_ACCOUNT_SEED (stable callout account; else
# minted per-boot) and OCIGER_NATS_WORKER_PASSWORD. With NONE set the bundle runs
# anonymous (no callout) — zero-config dispatch for adopters without an IdP.
# v3.9 Critical Isolation Alpha — opt the bundle into the role floor:
# the launcher appends `host all ck_participant 0.0.0.0/0 scram-sha-256`
# to pg_hba.conf on first initdb. Deploys MUST set
# OCIGER_CK_PARTICIPANT_PASSWORD before external exposure; without it,
# ck_participant exists but cannot log in (no default password — a default
# would make the role floor cosmetic per pgCK v3.9 §7).
ENV OCIGER_CK_PARTICIPANT_HBA=1
# (v0.7.30) OCIGER_DISABLE_OUTBOX_DRAIN removed — it only gated the deleted Go
# relay's drain. pgCK 0.4.24-nats owns the ckp.outbox drain in-process.
ENV S6_KEEP_ENV=1
ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2
ENV S6_VERBOSITY=1

EXPOSE 5432 8000 4222 9222
VOLUME ["/var/lib/postgresql/data"]

ENTRYPOINT ["/init"]
