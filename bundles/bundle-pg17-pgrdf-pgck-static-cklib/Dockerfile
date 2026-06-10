# syntax=docker/dockerfile:1.7
FROM ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.6 AS base
LABEL org.opencontainers.image.source="https://github.com/sporaxis-com/oci-germination"
LABEL org.opencontainers.image.description="CKP v3.8 static-only bundle — PG17 + pgRDF + pgCK + NATS WSS + ociger-static-server + cklib (no FastAPI/Python)"
LABEL org.opencontainers.image.licenses="MIT"
LABEL ck.bundle.role="prod"
LABEL ck.bundle.never-prod="false"

FROM ghcr.io/conceptkernel/ck-lib-js:1.4.0 AS cklib_source

# Rebuild ociger-supervisor + ociger-static-server from this repo's source
# so the supervisor knows the 'static' profile (the v0.1.2 base ships an
# older supervisor without staticProfile()).
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS go_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-supervisor/main.go ./cmd/ociger-supervisor/main.go
COPY cmd/ociger-static-server/main.go ./cmd/ociger-static-server/main.go
COPY internal/supervisor ./internal/supervisor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -o /out/ociger-supervisor ./cmd/ociger-supervisor && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -o /out/ociger-static-server ./cmd/ociger-static-server

FROM base
# Build-arg supplied by CI from the git tag; default `dev` keeps local
# `docker build` invocations clearly distinguished from a release.
ARG BUNDLE_VERSION=dev
LABEL org.opencontainers.image.version="${BUNDLE_VERSION}"
ENV OCIGER_SUPERVISOR_PROFILE=static

COPY --from=cklib_source / /app/cklib/
COPY --from=go_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor
COPY --from=go_build /out/ociger-static-server /usr/local/bin/ociger-static-server

EXPOSE 5432 8000 4222 9222
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
