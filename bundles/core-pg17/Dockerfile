# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS launcher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
COPY internal/launcher ./internal/launcher
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

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

FROM gcr.io/distroless/base-debian12:latest
ENV PGDATA=/var/lib/postgresql/data
COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]
