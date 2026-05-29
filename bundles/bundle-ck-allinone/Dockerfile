# syntax=docker/dockerfile:1.7
ARG TARGETARCH

# Postgres + pgRDF + pgCK + NATS base — provides /usr/lib/postgresql/17/*,
# /usr/share/postgresql/17/*, pgrdf.so + pgck.so + extension SQL/control files,
# nats-server, ociger-pg-launcher, ociger-supervisor, nats config. Selectively
# layered onto a scratch final image upstream of us.
FROM ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.4 AS pg_base

# Shape A additive — CK.Lib.Js static layer
FROM ghcr.io/conceptkernel/ck-lib-js:1.3.11 AS cklib_source

# Shape A additive — pgck-web FastAPI runtime. python:3.11-slim-based Debian
# image with /usr/local/bin/python3, /opt/venv (uvicorn + deps),
# /app/web (the consolidated web/ tree), /usr/local/bin/pgck-web-launcher.
# v0.2.6 ships per-arch tags only (no multi-arch index) per pgCK's pattern,
# so the tag interpolates TARGETARCH. Per pgCK's pgck-web RESPONSE this is
# the supported consumption shape.
FROM ghcr.io/styk-tv/pgck-web:v0.2.6-${TARGETARCH} AS pgckweb_base

# Rebuild the ociger-supervisor with fastapi-profile support. The pg_base
# image's supervisor doesn't know about OCIGER_SUPERVISOR_PROFILE=fastapi
# yet (that profile lands in this same commit).
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS supervisor_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-supervisor/main.go ./cmd/ociger-supervisor/main.go
COPY internal/supervisor ./internal/supervisor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -o /out/ociger-supervisor ./cmd/ociger-supervisor

# Final image — pgck-web (Debian + Python + uvicorn + /app/web + pgck-web-launcher)
# is the base; postgres + nats + pgRDF + pgCK + supervisor + cklib layer on top.
# This is the inversion of the previous design (which used a scratch+postgres base
# and tried to inline-printf a tiny FastAPI app — gap was that Python wasn't
# actually present in the final image; see _WIP/NOTIFIES.pgCK.pgck-web.v0.2.4.*).
FROM pgckweb_base
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination"
LABEL org.opencontainers.image.description="CKP v3.8 all-in-one — PG17 + pgRDF + pgCK + NATS + pgck-web (additive) + cklib"
LABEL org.opencontainers.image.licenses="MIT"

# Add postgres binaries + libs + extensions
COPY --from=pg_base /usr/lib/postgresql /usr/lib/postgresql
COPY --from=pg_base /usr/share/postgresql /usr/share/postgresql

# Add nats-server + config
COPY --from=pg_base /usr/local/bin/nats-server /usr/local/bin/nats-server
COPY --from=pg_base /etc/nats /etc/nats

# Add PG launcher
COPY --from=pg_base /usr/local/bin/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher

# Add the supervisor we just built (with fastapi profile)
COPY --from=supervisor_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor

# Add cklib at /app/cklib (alongside pgck-web's /app/web)
COPY --from=cklib_source / /app/cklib/

# postgres user 999 (matches pg_base's /etc/passwd convention) + writable dirs
RUN groupadd -g 999 -r postgres && \
    useradd -u 999 -g 999 -r -d /var/lib/postgresql -s /bin/sh postgres && \
    mkdir -p /var/lib/postgresql/data /var/run/postgresql && \
    chown -R postgres:postgres /var/lib/postgresql /var/run/postgresql && \
    chmod 700 /var/lib/postgresql/data

ENV PGDATA=/var/lib/postgresql/data
ENV OCIGER_SUPERVISOR_PROFILE=fastapi
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck

EXPOSE 5432 8000 4222 9222
VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
