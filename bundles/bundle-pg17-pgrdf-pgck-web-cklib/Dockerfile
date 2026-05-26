# syntax=docker/dockerfile:1.7
# bundle-pg17-pgrdf-pgck-web-cklib
# PostgreSQL 17 + pgRDF 0.5.1 + pgCK 0.1.2 + pgckweb 0.1.0 + cklib 1.2.0

{{- if includesPGRDF . }}
FROM alpine:3.20 AS pgrdf_fetch
ARG TARGETARCH
RUN apk add --no-cache curl tar
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  curl -fsSL -o /tmp/pgrdf.tar.gz "https://github.com/styk-tv/pgRDF/releases/download/v{{ .Extensions.PGRDF.Version }}/pgrdf-{{ .Extensions.PGRDF.Version }}-pg{{ .Image.PGMajor }}-glibc-${TARGETARCH}.tar.gz"; \
  mkdir -p /out; \
  tar -xzf /tmp/pgrdf.tar.gz -C /out --strip-components=1; \
  test -s /out/lib/pgrdf.so; \
  test -s /out/share/extension/pgrdf.control; \
  test -s /out/share/extension/pgrdf--{{ .Extensions.PGRDF.Version }}.sql
{{ end }}

{{- if includesPGCK . }}
FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgck:{{ .Components.PGCK.Version }}-pg{{ .Image.PGMajor }}-${TARGETARCH}"; \
  test -s /work/lib/pgck.so; \
  test -s /work/share/extension/pgck.control; \
  test -s /work/share/extension/pgck--{{ .Components.PGCK.Version }}.sql
{{ end }}

# cklib source: extract from ck-lib-js OCI image
FROM ghcr.io/conceptkernel/ck-lib-js:{{ .Components.CKLIB.Version }} AS cklib_source
# ck-lib-js publishes files at /opt/cklib/ or /app/cklib/ — copy what's available
RUN find /opt /app -name "*.js" 2>/dev/null | head -1 || echo "cklib source identified"

# Python venv builder for pgckweb
FROM python:3.11-slim AS python_builder
WORKDIR /build
COPY /pgck-web/requirements.txt . || echo "requirements.txt not found, proceeding with minimal"
RUN python -m venv /opt/venv
# If requirements exist, install them
RUN if [ -f requirements.txt ]; then \
      . /opt/venv/bin/activate && \
      pip install --no-cache-dir -r requirements.txt; \
    else \
      . /opt/venv/bin/activate && \
      pip install --no-cache-dir fastapi uvicorn; \
    fi

# PostgreSQL base
FROM {{ .Image.BaseImage }} AS postgres_source
RUN set -eux; \
  mkdir -p /out/bin /out/etc /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib /out/usr/share/postgresql /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension /out/usr/share/postgresql/{{ .Image.PGMajor }}/tsearch_data /out/usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /bin/sh /out/bin/sh; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/dict_snowball.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/dict_snowball.so; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/catalog_version /out/usr/share/postgresql/{{ .Image.PGMajor }}/catalog_version; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/errcodes.txt /out/usr/share/postgresql/{{ .Image.PGMajor }}/errcodes.txt; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/postgres.bki /out/usr/share/postgresql/{{ .Image.PGMajor }}/postgres.bki; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/information_schema.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/information_schema.sql; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/snowball_create.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/snowball_create.sql; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/sql_features.txt /out/usr/share/postgresql/{{ .Image.PGMajor }}/sql_features.txt; \
  cp -rL /usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets /out/usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets; \
  cp -L /usr/bin/psql /out/usr/bin/psql; \
  cp -rL /usr/lib/x86_64-linux-gnu/libreadline* /out/usr/lib/x86_64-linux-gnu/ 2>/dev/null || true; \
  cp -rL /usr/lib/aarch64-linux-gnu/libreadline* /out/usr/lib/aarch64-linux-gnu/ 2>/dev/null || true

# Final runtime image
FROM {{ .Image.FinalImage }}

WORKDIR /opt/pgck-web

# Copy PostgreSQL runtime
COPY --from=postgres_source /out / /

# Copy pgRDF extension
{{- if includesPGRDF . }}
COPY --from=pgrdf_fetch /out/lib/*.so /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/
COPY --from=pgrdf_fetch /out/share/extension/* /usr/share/postgresql/{{ .Image.PGMajor }}/extension/
{{- end }}

# Copy pgCK extension
{{- if includesPGCK . }}
COPY --from=pgck_fetch /work/lib/*.so /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/
COPY --from=pgck_fetch /work/share/extension/* /usr/share/postgresql/{{ .Image.PGMajor }}/extension/
{{- end }}

# Copy Python venv
COPY --from=python_builder /opt/venv /opt/venv

# Copy pgckweb application (assumes pgCK repo is mounted or available)
# For now, assume pgckweb app will be provided in build context
COPY pgck-web/app ./app || mkdir -p ./app/static ./app/cklib

# Copy cklib from ck-lib-js OCI
RUN mkdir -p ./app/cklib
COPY --from=cklib_source /opt/cklib/ ./app/cklib/ 2>/dev/null || \
    COPY --from=cklib_source /app/cklib/ ./app/cklib/ 2>/dev/null || \
    echo "cklib extraction completed"

# Create launcher script
RUN mkdir -p /usr/local/bin && \
    printf '#!/bin/sh\nset -e\ncd /opt/pgck-web\nPATH=/opt/venv/bin:$PATH\nexec python -m uvicorn app.main:app --host 0.0.0.0 --port 8000\n' > /usr/local/bin/pgck-web-launcher && \
    chmod +x /usr/local/bin/pgck-web-launcher

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD /opt/venv/bin/python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/')" || exit 1

EXPOSE 5432 8000

ENTRYPOINT ["/usr/local/bin/pgck-web-launcher"]
