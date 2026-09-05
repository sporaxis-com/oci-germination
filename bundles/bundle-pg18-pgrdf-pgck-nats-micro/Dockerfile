# syntax=docker/dockerfile:1.7
# v0.2.2: the source commit, stamped into every arch as
# org.opencontainers.image.revision (CI's cross-arch parity gate asserts all
# published arches share one revision). Also drops the bookworm Go builder —
# golang:1.24-trixie ships the same go1.24.13 toolchain, and with CGO_ENABLED=0
# the builder's userland contributes nothing to the scratch runtime.
ARG GIT_SHA=unknown
FROM --platform=$BUILDPLATFORM golang:1.24-trixie AS launcher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
COPY internal/launcher ./internal/launcher
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher
FROM --platform=$BUILDPLATFORM golang:1.24-trixie AS supervisor_build
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
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgrdf-bundle:0.6.34-pg18-${TARGETARCH}"; \
  mkdir -p /out; \
  tar -xzf "pgrdf-0.6.34-pg18-glibc-${TARGETARCH}.tar.gz" -C /out --strip-components=1; \
  test -s /out/lib/pgrdf.so; \
  test -s /out/share/extension/pgrdf.control; \
  test -s /out/share/extension/pgrdf--0.6.34.sql  # pgRDF v0.5.25+ aligned the SQL filename with the release tag (Cargo.toml + control + META.json all bumped together)


FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgck:0.4.112-pg18-nats-${TARGETARCH}"; \
  test -s /work/lib/pgck.so; \
  test -s /work/share/extension/pgck.control; \
  test -s /work/share/extension/pgck--0.4.112.sql; \
  # CKP v3.11 ships a versioned ontology tree — core plus the wave: and lex:
  # modules a kernel adopts through a sealed core#Adoption naming each module's
  # sourceDigest. ck-allinone's init.sql seals exactly those Adoptions, so the
  # digests below are LOAD-BEARING: they must equal versions.yaml `modules:`
  # (the smoke pin-ledger gate compares what sealed against that same source).
  # 0.4.82 ships BOTH trees: v3.11 (core/wave/lexicon byte-stable since 0.4.77,
  # plus the NEW recon module) and v3.12 FINAL (boot's default root). 0.4.109
  # changes v3.12/core.ttl ONLY — v3.11 and v3.12 wave/recon are byte-stable
  # through the whole 0.4.89..0.4.109 wave, AND unchanged again by 0.4.111 —
  # which is behaviour-only, as is 0.4.112 (the digest-ladder PRE/AT rungs).
  # All seven re-measured from pgck:0.4.112-pg18-nats-{amd64,arm64} 2026-09-05, identical
  # to 0.4.109's and byte-identical across
  # arches; a mismatch here means upstream shipped different bytes than we
  # pinned, and the build MUST die rather than seal or ground an unvouched root.
  echo "e5f7d1e54b32fa0ba2d41ba248e0909b96ee1ebb4344e2d9e9ccdf4e0b25348d  /work/ontology/v3.11/core.ttl" | sha256sum -c -; \
  echo "f4ad27cec4417e2ed5b566adb7f7ee200b3c3fbfddf25adf840d267fd57e417b  /work/ontology/v3.11/modules/wave.ttl" | sha256sum -c -; \
  echo "5def86bacfe3d2a8a6a85e9c81f8f9d77c3cfeee72509a360b2b00433d3cf21c  /work/ontology/v3.11/modules/lexicon.ttl" | sha256sum -c -; \
  echo "b2b11f1b76e22f7bfac10be3eba7b5104ff0d5d0a1d92147ec3dc392f1475d7d  /work/ontology/v3.11/modules/recon.ttl" | sha256sum -c -; \
  echo "2e90a6512c7aae567714be69b92b37af4393e8da8d65a05c047c5d51e1bed07b  /work/ontology/v3.12/core.ttl" | sha256sum -c -; \
  echo "ad887db28c6e0ea04c7cbd835c40dc5441f073be988475a9634c76e9131db727  /work/ontology/v3.12/modules/wave.ttl" | sha256sum -c -; \
  echo "6a7c199e7ad19580b5e975624d15a9d4823e20dee9946cd20600b04c509100ea  /work/ontology/v3.12/modules/recon.ttl" | sha256sum -c -


FROM nats:2.14.2-scratch AS nats_source


FROM postgres:18-trixie AS postgres_source
RUN set -eux; \
  mkdir -p /out/bin /out/etc /out/usr/lib/postgresql/18/bin /out/usr/lib/postgresql/18/lib /out/usr/share/postgresql /out/usr/share/postgresql/18/extension /out/usr/share/postgresql/18/tsearch_data /out/usr/share/postgresql/18/timezonesets /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /bin/sh /out/bin/sh; \
  cp -L /usr/lib/postgresql/18/bin/postgres /out/usr/lib/postgresql/18/bin/postgres; \
  cp -L /usr/lib/postgresql/18/bin/initdb /out/usr/lib/postgresql/18/bin/initdb; \
  cp -L /usr/lib/postgresql/18/lib/dict_snowball.so /out/usr/lib/postgresql/18/lib/dict_snowball.so; \
  cp -L /usr/lib/postgresql/18/lib/plpgsql.so /out/usr/lib/postgresql/18/lib/plpgsql.so; \
  cp -L /usr/share/postgresql/18/catalog_version /out/usr/share/postgresql/18/catalog_version; \
  cp -L /usr/share/postgresql/18/errcodes.txt /out/usr/share/postgresql/18/errcodes.txt; \
  cp -L /usr/share/postgresql/18/postgres.bki /out/usr/share/postgresql/18/postgres.bki; \
  cp -L /usr/share/postgresql/18/information_schema.sql /out/usr/share/postgresql/18/information_schema.sql; \
  cp -L /usr/share/postgresql/18/snowball_create.sql /out/usr/share/postgresql/18/snowball_create.sql; \
  cp -L /usr/share/postgresql/18/sql_features.txt /out/usr/share/postgresql/18/sql_features.txt; \
  cp -L /usr/share/postgresql/18/system_functions.sql /out/usr/share/postgresql/18/system_functions.sql; \
  cp -L /usr/share/postgresql/18/system_views.sql /out/usr/share/postgresql/18/system_views.sql; \
  cp -L /usr/share/postgresql/18/system_constraints.sql /out/usr/share/postgresql/18/system_constraints.sql; \
  cp -a /usr/share/postgresql/18/tsearch_data/. /out/usr/share/postgresql/18/tsearch_data/; \
  cp -a /usr/share/postgresql/18/timezonesets/. /out/usr/share/postgresql/18/timezonesets/; \
  cp -L /usr/share/postgresql/postgresql.conf.sample /out/usr/share/postgresql/postgresql.conf.sample; \
  cp -L /usr/share/postgresql/18/postgresql.conf.sample /out/usr/share/postgresql/18/postgresql.conf.sample; \
  cp -L /usr/share/postgresql/18/pg_hba.conf.sample /out/usr/share/postgresql/18/pg_hba.conf.sample; \
  cp -L /usr/share/postgresql/18/pg_ident.conf.sample /out/usr/share/postgresql/18/pg_ident.conf.sample; \
  cp -L /usr/share/postgresql/18/pg_service.conf.sample /out/usr/share/postgresql/18/pg_service.conf.sample; \
  cp -L /usr/share/postgresql/18/psqlrc.sample /out/usr/share/postgresql/18/psqlrc.sample; \
  cp -L /usr/share/postgresql/18/extension/plpgsql.control /out/usr/share/postgresql/18/extension/plpgsql.control; \
  cp -L /usr/share/postgresql/18/extension/plpgsql--*.sql /out/usr/share/postgresql/18/extension/; \
  printf 'root:x:0:0:root:/root:/bin/sh\npostgres:x:999:999:postgres:/var/lib/postgresql:/bin/sh\nnats:x:998:998:nats:/nonexistent:/bin/false\nhttpd:x:997:997:httpd:/nonexistent:/bin/false\n' > /out/etc/passwd; \
  printf 'root:x:0:\npostgres:x:999:\nnats:x:998:\nhttpd:x:997:\n' > /out/etc/group; \
  ldd /usr/lib/postgresql/18/bin/postgres | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/18/bin/initdb | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/18/lib/plpgsql.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/18/lib/dict_snowball.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out
COPY --from=pgrdf_fetch /out/lib/pgrdf.so /out/usr/lib/postgresql/18/lib/pgrdf.so
COPY --from=pgrdf_fetch /out/share/extension/ /out/usr/share/postgresql/18/extension/

COPY --from=pgck_fetch /work/lib/pgck.so /out/usr/lib/postgresql/18/lib/pgck.so
COPY --from=pgck_fetch /work/share/extension/ /out/usr/share/postgresql/18/extension/
# pgCK v0.4.2+ ships its ontology modules WITH the extension artifact (their
# install-cascade ask-3 fix). /ontology is the documented default root for
# ckp.boot() / ckp.import_module(). This makes the artifact the canonical
# fixture source — downstream bundles no longer vendor their own copies.
COPY --from=pgck_fetch /work/ontology/ /out/ontology/

RUN set -eux; \
  if [ -f /out/usr/lib/postgresql/18/lib/pgrdf.so ]; then \
    ldd /out/usr/lib/postgresql/18/lib/pgrdf.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  fi; \
  if [ -f /out/usr/lib/postgresql/18/lib/pgck.so ]; then \
    cp -L /usr/lib/postgresql/18/lib/pgcrypto.so /out/usr/lib/postgresql/18/lib/pgcrypto.so; \
    cp -L /usr/share/postgresql/18/extension/pgcrypto.control /out/usr/share/postgresql/18/extension/pgcrypto.control; \
    cp -L /usr/share/postgresql/18/extension/pgcrypto--*.sql /out/usr/share/postgresql/18/extension/; \
    ldd /out/usr/lib/postgresql/18/lib/pgck.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
    ldd /out/usr/lib/postgresql/18/lib/pgcrypto.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  fi


FROM scratch
ARG GIT_SHA
LABEL org.opencontainers.image.revision="${GIT_SHA}"
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
COPY bundles/bundle-pg18-pgrdf-pgck-nats-micro/nats-server.conf /etc/nats/nats-server.conf
EXPOSE 5432 4222 9222

VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
