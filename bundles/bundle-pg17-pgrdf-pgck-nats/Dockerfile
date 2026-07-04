# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS launcher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
COPY internal/launcher ./internal/launcher
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS supervisor_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-supervisor/main.go ./cmd/ociger-supervisor/main.go
COPY internal/supervisor ./internal/supervisor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-supervisor ./cmd/ociger-supervisor


FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgrdf_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgrdf-bundle:0.6.19-pg17-${TARGETARCH}"; \
  mkdir -p /out; \
  tar -xzf "pgrdf-0.6.19-pg17-glibc-${TARGETARCH}.tar.gz" -C /out --strip-components=1; \
  test -s /out/lib/pgrdf.so; \
  test -s /out/share/extension/pgrdf.control; \
  test -s /out/share/extension/pgrdf--0.6.19.sql  # pgRDF v0.5.25+ aligned the SQL filename with the release tag (Cargo.toml + control + META.json all bumped together)


FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgck:0.4.17-pg17-${TARGETARCH}"; \
  test -s /work/lib/pgck.so; \
  test -s /work/share/extension/pgck.control; \
  test -s /work/share/extension/pgck--0.4.17.sql


FROM nats:2.14.2-scratch AS nats_source


FROM postgres:17-bookworm AS postgres_source
RUN set -eux; \
  mkdir -p /out/bin /out/usr/lib/postgresql /out/usr/share/postgresql /out/etc /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /bin/sh /out/bin/sh; \
  cp -a /usr/lib/postgresql/17 /out/usr/lib/postgresql/; \
  cp -a /usr/share/postgresql/17 /out/usr/share/postgresql/; \
  cp -L /usr/share/postgresql/postgresql.conf.sample /out/usr/share/postgresql/postgresql.conf.sample; \
  cp /etc/passwd /out/etc/passwd; \
  cp /etc/group /out/etc/group; \
  ldd /usr/lib/postgresql/17/bin/postgres | tr ' ' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/17/bin/initdb | tr ' ' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out
COPY --from=pgrdf_fetch /out/lib/pgrdf.so /out/usr/lib/postgresql/17/lib/pgrdf.so
COPY --from=pgrdf_fetch /out/share/extension/ /out/usr/share/postgresql/17/extension/

COPY --from=pgck_fetch /work/lib/pgck.so /out/usr/lib/postgresql/17/lib/pgck.so
COPY --from=pgck_fetch /work/share/extension/ /out/usr/share/postgresql/17/extension/


FROM gcr.io/distroless/base-debian12:latest
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination"
LABEL org.opencontainers.image.licenses="MIT"
LABEL ck.bundle.role="prod"
LABEL ck.bundle.never-prod="false"
ENV PGDATA=/var/lib/postgresql/data
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck

COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
COPY --from=supervisor_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor
COPY --from=nats_source /nats-server /usr/local/bin/nats-server
COPY bundles/bundle-pg17-pgrdf-pgck-nats/nats-server.conf /etc/nats/nats-server.conf
EXPOSE 5432 4222 9222

VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
