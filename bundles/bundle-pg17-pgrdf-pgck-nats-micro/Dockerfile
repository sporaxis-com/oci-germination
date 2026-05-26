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


FROM alpine:3.20 AS pgrdf_fetch
ARG TARGETARCH
RUN apk add --no-cache curl tar
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  curl -fsSL -o /tmp/pgrdf.tar.gz "https://github.com/styk-tv/pgRDF/releases/download/v0.5.1/pgrdf-0.5.1-pg17-glibc-${TARGETARCH}.tar.gz"; \
  mkdir -p /out; \
  tar -xzf /tmp/pgrdf.tar.gz -C /out --strip-components=1; \
  test -s /out/lib/pgrdf.so; \
  test -s /out/share/extension/pgrdf.control; \
  test -s /out/share/extension/pgrdf--0.5.1.sql


FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgck:0.1.2-pg17-${TARGETARCH}"; \
  test -s /work/lib/pgck.so; \
  test -s /work/share/extension/pgck.control; \
  test -s /work/share/extension/pgck--0.1.2.sql


FROM nats:2.14.1-scratch AS nats_source


FROM postgres:17-bookworm AS postgres_source
RUN set -eux; \
  mkdir -p /out/bin /out/etc /out/usr/lib/postgresql/17/bin /out/usr/lib/postgresql/17/lib /out/usr/share/postgresql /out/usr/share/postgresql/17/extension /out/usr/share/postgresql/17/tsearch_data /out/usr/share/postgresql/17/timezonesets /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /bin/sh /out/bin/sh; \
  cp -L /usr/lib/postgresql/17/bin/postgres /out/usr/lib/postgresql/17/bin/postgres; \
  cp -L /usr/lib/postgresql/17/bin/initdb /out/usr/lib/postgresql/17/bin/initdb; \
  cp -L /usr/lib/postgresql/17/lib/dict_snowball.so /out/usr/lib/postgresql/17/lib/dict_snowball.so; \
  cp -L /usr/lib/postgresql/17/lib/plpgsql.so /out/usr/lib/postgresql/17/lib/plpgsql.so; \
  cp -L /usr/share/postgresql/17/catalog_version /out/usr/share/postgresql/17/catalog_version; \
  cp -L /usr/share/postgresql/17/errcodes.txt /out/usr/share/postgresql/17/errcodes.txt; \
  cp -L /usr/share/postgresql/17/postgres.bki /out/usr/share/postgresql/17/postgres.bki; \
  cp -L /usr/share/postgresql/17/information_schema.sql /out/usr/share/postgresql/17/information_schema.sql; \
  cp -L /usr/share/postgresql/17/snowball_create.sql /out/usr/share/postgresql/17/snowball_create.sql; \
  cp -L /usr/share/postgresql/17/sql_features.txt /out/usr/share/postgresql/17/sql_features.txt; \
  cp -L /usr/share/postgresql/17/system_functions.sql /out/usr/share/postgresql/17/system_functions.sql; \
  cp -L /usr/share/postgresql/17/system_views.sql /out/usr/share/postgresql/17/system_views.sql; \
  cp -L /usr/share/postgresql/17/system_constraints.sql /out/usr/share/postgresql/17/system_constraints.sql; \
  cp -a /usr/share/postgresql/17/tsearch_data/. /out/usr/share/postgresql/17/tsearch_data/; \
  cp -a /usr/share/postgresql/17/timezonesets/. /out/usr/share/postgresql/17/timezonesets/; \
  cp -L /usr/share/postgresql/postgresql.conf.sample /out/usr/share/postgresql/postgresql.conf.sample; \
  cp -L /usr/share/postgresql/17/postgresql.conf.sample /out/usr/share/postgresql/17/postgresql.conf.sample; \
  cp -L /usr/share/postgresql/17/pg_hba.conf.sample /out/usr/share/postgresql/17/pg_hba.conf.sample; \
  cp -L /usr/share/postgresql/17/pg_ident.conf.sample /out/usr/share/postgresql/17/pg_ident.conf.sample; \
  cp -L /usr/share/postgresql/17/pg_service.conf.sample /out/usr/share/postgresql/17/pg_service.conf.sample; \
  cp -L /usr/share/postgresql/17/psqlrc.sample /out/usr/share/postgresql/17/psqlrc.sample; \
  cp -L /usr/share/postgresql/17/extension/plpgsql.control /out/usr/share/postgresql/17/extension/plpgsql.control; \
  cp -L /usr/share/postgresql/17/extension/plpgsql--*.sql /out/usr/share/postgresql/17/extension/; \
  printf 'root:x:0:0:root:/root:/bin/sh\npostgres:x:999:999:postgres:/var/lib/postgresql:/bin/sh\n' > /out/etc/passwd; \
  printf 'root:x:0:\npostgres:x:999:\n' > /out/etc/group; \
  ldd /usr/lib/postgresql/17/bin/postgres | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/17/bin/initdb | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/17/lib/plpgsql.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/17/lib/dict_snowball.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out
COPY --from=pgrdf_fetch /out/lib/pgrdf.so /out/usr/lib/postgresql/17/lib/pgrdf.so
COPY --from=pgrdf_fetch /out/share/extension/ /out/usr/share/postgresql/17/extension/

COPY --from=pgck_fetch /work/lib/pgck.so /out/usr/lib/postgresql/17/lib/pgck.so
COPY --from=pgck_fetch /work/share/extension/ /out/usr/share/postgresql/17/extension/

RUN set -eux; \
  if [ -f /out/usr/lib/postgresql/17/lib/pgrdf.so ]; then \
    ldd /out/usr/lib/postgresql/17/lib/pgrdf.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  fi; \
  if [ -f /out/usr/lib/postgresql/17/lib/pgck.so ]; then \
    cp -L /usr/lib/postgresql/17/lib/pgcrypto.so /out/usr/lib/postgresql/17/lib/pgcrypto.so; \
    cp -L /usr/share/postgresql/17/extension/pgcrypto.control /out/usr/share/postgresql/17/extension/pgcrypto.control; \
    cp -L /usr/share/postgresql/17/extension/pgcrypto--*.sql /out/usr/share/postgresql/17/extension/; \
    ldd /out/usr/lib/postgresql/17/lib/pgck.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
    ldd /out/usr/lib/postgresql/17/lib/pgcrypto.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  fi


FROM scratch
ENV PGDATA=/var/lib/postgresql/data
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgck

COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
COPY --from=supervisor_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor
COPY --from=nats_source /nats-server /usr/local/bin/nats-server
COPY bundles/bundle-pg17-pgrdf-pgck-nats-micro/nats-server.conf /etc/nats/nats-server.conf
EXPOSE 5432 4222 9222

VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
