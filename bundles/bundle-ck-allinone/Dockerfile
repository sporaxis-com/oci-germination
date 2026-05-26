# syntax=docker/dockerfile:1.7
# bundle-ck-allinone
# CKP v3.8 all-in-one: PostgreSQL 17 (micro) + pgRDF 0.5.1 + pgCK 0.1.2 + pgckweb 0.1.0 + cklib 1.2.0 + NATS 2.14.1 + supervisor

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

# NATS source
{{- if hasNATS . }}
FROM {{ .Services.NATS.SourceImage }} AS nats_source
{{ end }}

# Supervisor builder (Go)
{{- if hasNATS . }}
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS supervisor_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-supervisor/main.go ./cmd/ociger-supervisor/main.go
COPY internal/supervisor ./internal/supervisor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-supervisor ./cmd/ociger-supervisor
{{ end }}

# NATS config generator
{{- if hasNATS . }}
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS nats_config_builder
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-nats-config/main.go ./cmd/ociger-nats-config/main.go 2>/dev/null || echo "nats-config tool not found"
COPY internal/nats ./internal/nats 2>/dev/null || mkdir -p ./internal/nats
RUN if [ -f cmd/ociger-nats-config/main.go ]; then \
      CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-nats-config ./cmd/ociger-nats-config; \
    else \
      echo "Using default nats-server.conf"; \
    fi
{{ end }}

# Python venv builder for pgckweb
FROM python:3.11-slim AS python_builder
WORKDIR /build
# Assume pgckweb code is provided, or install minimal
RUN python -m venv /opt/venv && \
    . /opt/venv/bin/activate && \
    pip install --no-cache-dir fastapi uvicorn

# PostgreSQL micro runtime
FROM {{ .Image.BaseImage }} AS postgres_source
RUN set -eux; \
  mkdir -p /out/bin /out/etc /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib /out/usr/share/postgresql /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension /out/usr/share/postgresql/{{ .Image.PGMajor }}/tsearch_data /out/usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /bin/sh /out/bin/sh; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/dict_snowball.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/dict_snowball.so 2>/dev/null || true; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so 2>/dev/null || true; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/catalog_version /out/usr/share/postgresql/{{ .Image.PGMajor }}/catalog_version 2>/dev/null || true; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/errcodes.txt /out/usr/share/postgresql/{{ .Image.PGMajor }}/errcodes.txt 2>/dev/null || true; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/postgres.bki /out/usr/share/postgresql/{{ .Image.PGMajor }}/postgres.bki 2>/dev/null || true; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/information_schema.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/information_schema.sql 2>/dev/null || true; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/snowball_create.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/snowball_create.sql 2>/dev/null || true; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/sql_features.txt /out/usr/share/postgresql/{{ .Image.PGMajor }}/sql_features.txt 2>/dev/null || true; \
  cp -rL /usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets /out/usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets 2>/dev/null || true; \
  cp -L /usr/bin/psql /out/usr/bin/psql 2>/dev/null || true

# Final runtime image
FROM {{ .Image.FinalImage }}

WORKDIR /opt/pgck-web

# Copy PostgreSQL runtime (micro stripped)
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

# Copy pgckweb application
COPY pgck-web/app ./app 2>/dev/null || mkdir -p ./app/static ./app/cklib

# Copy cklib from ck-lib-js OCI
RUN mkdir -p ./app/cklib
COPY --from=cklib_source /opt/cklib/ ./app/cklib/ 2>/dev/null || \
    COPY --from=cklib_source /app/cklib/ ./app/cklib/ 2>/dev/null || \
    echo "cklib extraction completed"

# Copy NATS binary
{{- if hasNATS . }}
COPY --from=nats_source /usr/sbin/nats-server /usr/local/bin/nats-server
RUN chmod +x /usr/local/bin/nats-server
{{- end }}

# Copy supervisor binary
{{- if hasNATS . }}
COPY --from=supervisor_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor
RUN chmod +x /usr/local/bin/ociger-supervisor
{{- end }}

# Create NATS config directory
{{- if hasNATS . }}
RUN mkdir -p /etc/nats
RUN printf 'port: {{ .Services.NATS.CorePort }}\nhttp_port: 8222\nwebsocket {\n  port: {{ .Services.NATS.WebSocketPort }}\n  same_origin: false\n  compression: auto\n}\n' > /etc/nats/nats-server.conf
{{- end }}

# Create launcher script for supervisor
{{- if hasNATS . }}
RUN mkdir -p /usr/local/bin && \
    printf '#!/bin/sh\nset -e\nexec /usr/local/bin/ociger-supervisor\n' > /usr/local/bin/ck-allinone-launcher && \
    chmod +x /usr/local/bin/ck-allinone-launcher
{{- end }}

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD /opt/venv/bin/python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/')" || exit 1

EXPOSE 5432 8000 {{ .Services.NATS.CorePort }} {{ .Services.NATS.WebSocketPort }}

{{- if hasNATS . }}
ENTRYPOINT ["/usr/local/bin/ck-allinone-launcher"]
{{- else }}
ENTRYPOINT ["/usr/local/bin/pgck-web-launcher"]
{{- end }}
