# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS launcher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

FROM postgres:17-bookworm AS postgres_source
RUN set -eux; \
  mkdir -p /out/usr/lib/postgresql /out/usr/share/postgresql /out/etc /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -a /usr/lib/postgresql/17 /out/usr/lib/postgresql/; \
  cp -a /usr/share/postgresql/17 /out/usr/share/postgresql/; \
  cp /etc/passwd /out/etc/passwd; \
  cp /etc/group /out/etc/group; \
  ldd /usr/lib/postgresql/17/bin/postgres | awk '/=> \\/|^\\// {print $(NF-1)}' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/17/bin/initdb | awk '/=> \\/|^\\// {print $(NF-1)}' | sort -u | xargs -r -I '{}' cp --parents '{}' /out

FROM gcr.io/distroless/base-debian12:latest
ENV PGDATA=/var/lib/postgresql/data
COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]
