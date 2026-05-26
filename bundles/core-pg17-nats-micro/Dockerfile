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

FROM scratch
ENV PGDATA=/var/lib/postgresql/data
COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
COPY --from=supervisor_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor
COPY --from=nats_source /nats-server /usr/local/bin/nats-server
COPY bundles/core-pg17-nats-micro/nats-server.conf /etc/nats/nats-server.conf
EXPOSE 5432 4222 9222

VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
