# syntax=docker/dockerfile:1.7
#
# ociger-pg17-bookworm-pgrdf — PostgreSQL 17 + pgRDF on the STOCK
# postgres:17-bookworm base. The operable, zero-maintenance counterpart to
# the size-minimal distroless `ociger-pg17-pgrdf`:
#
#   • full client in the image — psql / pg_isready on PATH, so
#     `docker exec <c> psql …` and `docker exec <c> pg_isready` just work
#     (a benchmark runner that drives the DB by docker exec needs this).
#   • pgRDF preloaded + the extension AUTO-CREATED on first boot — after
#     `docker run` there is nothing to do: `SELECT pgrdf.version()` works
#     with no CREATE EXTENSION step.
#
# This is the clean pgRDF-only benchmark image: shared_preload_libraries is
# pgrdf alone (no pgCK hooks to skew numbers), on exactly the base pgRDF's
# own LUBM benchmark uses (stock postgres:17-bookworm).
#
# Layer-addition only (this repo never compiles): pgRDF comes from the
# attested upstream OCI artifact via oras.

# Supplied by CI from the git tag; `dev` marks a local build.
ARG BUNDLE_VERSION=dev

# ─── Stage 1: fetch the attested pgRDF extension files ────────────────────
# Literal version pins (not an ARG) so versions.yaml's drift gate
# (scripts/check-versions.sh) can see + enforce them, same as the sibling
# pgRDF bundles. Bump via versions.yaml → check-versions → here.
FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgrdf_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgrdf-bundle:0.6.19-pg17-${TARGETARCH}"; \
  mkdir -p /out; \
  tar -xzf "pgrdf-0.6.17-pg17-glibc-${TARGETARCH}.tar.gz" -C /out --strip-components=1; \
  test -s /out/lib/pgrdf.so; \
  test -s /out/share/extension/pgrdf.control; \
  test -s /out/share/extension/pgrdf--0.6.19.sql

# ─── Final: stock postgres + pgRDF, ready with zero post-start work ───────
FROM postgres:17-bookworm
ARG BUNDLE_VERSION
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination" \
      org.opencontainers.image.description="PostgreSQL 17 + pgRDF on stock bookworm — full client (psql/pg_isready on PATH), pgRDF preloaded and auto-created on first boot. Zero maintenance after docker run. The clean pgRDF-only benchmark image." \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${BUNDLE_VERSION}" \
      ck.bundle.role="prod" \
      ck.bundle.never-prod="false"

# pgRDF runtime + extension files into the stock postgres tree.
COPY --from=pgrdf_fetch /out/lib/pgrdf.so /usr/lib/postgresql/17/lib/pgrdf.so
COPY --from=pgrdf_fetch /out/share/extension/ /usr/share/postgresql/17/extension/

# Bake the preload into the conf TEMPLATE so initdb generates a
# postgresql.conf that already loads pgrdf — which means even the
# entrypoint's transient init server has pgrdf available, so the
# CREATE EXTENSION in /docker-entrypoint-initdb.d below succeeds, and the
# real server inherits it. No env, no ALTER SYSTEM, no restart needed.
RUN echo "shared_preload_libraries = 'pgrdf'" >> /usr/share/postgresql/17/postgresql.conf.sample

# Auto-create the extension on first initdb. Runs once, on a fresh volume,
# against the (already-preloaded) transient server — so a plain `docker run`
# yields a database with pgrdf installed and ready.
COPY bundles/bundle-pg17-bookworm-pgrdf/initdb/10-pgrdf.sql /docker-entrypoint-initdb.d/10-pgrdf.sql

# Inherits stock postgres ENTRYPOINT/CMD/EXPOSE 5432 — POSTGRES_PASSWORD etc.
# all behave exactly as the upstream image documents.
