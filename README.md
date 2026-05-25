# OCI Germination

`oci-germination` is a public experiment in building very small OCI-delivered PostgreSQL bundles without rebuilding upstream extension projects here.

The immediate release target is a three-part bundle:

1. PostgreSQL 17 as a minimal embedded runtime
2. `pgRDF` consumed as an upstream OCI artifact
3. `pgCK` consumed as an upstream OCI artifact

This repo owns the assembly surface, size discipline, release pipeline, and documentation. It does **not** own the `pgRDF` or `pgCK` build pipelines.

## Current Status

The current shipped layer is `core-pg17-min`:

- PostgreSQL 17 extracted from `postgres:17-bookworm`
- distroless final image based on `gcr.io/distroless/base-debian12:latest`
- tiny Go launcher for first-boot `initdb`
- verified local bind-mount persistence
- generator-driven Dockerfile and Bake output from `bundles/core-pg17/bundle.yaml`

The critical next release target remains:

- `pg17 + pgRDF + pgCK` in one runnable bundle image

## One-Line Launch

Latest launch:

```bash
docker run --rm -d \
  --name ociger-core-pg17 \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-core-pg17-data:/var/lib/postgresql/data" \
  -p 15432:5432 \
  ghcr.io/sporaxis-com/ociger-core-pg17-min:latest
```

Then connect from the host:

```bash
psql -h 127.0.0.1 -p 15432 -U postgres -d postgres
```

Notes:

- Host port `15432` avoids clobbering an existing local `5432`.
- Current defaults are for local/dev use, not production hardening.
- The image initializes its data directory on first boot if `PGDATA` is empty.
- Replace `:latest` with any `core-pg17-vX.Y.Z` tag when you want a pinned release.

## Current Core Specification

`core-pg17-min` is the first runnable layer in the germination sequence.

Design constraints:

- keep the final runtime close to distroless
- use ordinary multi-stage Docker builds, not a custom layer assembler
- generate the Dockerfile and Bake file from a small checked-in bundle spec
- keep local verification contained to `ociger-`-prefixed resources
- prepare the build surface so future bundles can pull `pgRDF` and `pgCK` OCI artifacts during image assembly

This repository does **not** rebuild `pgRDF` or `pgCK`. Those stay upstream. This repository will later consume their published OCI artifacts.

## Measured Core Footprint

Verified on local `linux/arm64` Colima on May 25, 2026:

- Uncompressed image size: `146277807` bytes (`139.5 MiB`)
- Compressed local archive size: `55630530` bytes (`53.1 MiB`)
- Base source image: `postgres:17-bookworm`
- Final runtime base: `gcr.io/distroless/base-debian12:latest`
- PostgreSQL version: `17.10`

These are measured values from actual builds, not estimates.

## Persistence Proof

The local smoke test does all of the following against the built image:

1. boots PostgreSQL in a bind-mounted `PGDATA`
2. creates database `ociger_demo`
3. creates table `public.demo_rows`
4. inserts a row
5. queries it back
6. resolves the relation file with `pg_relation_filepath(...)`
7. confirms the file exists on the host bind mount

Example verification output:

```text
CREATE DATABASE
CREATE TABLE
INSERT 0 1
 id |       note
----+------------------
  1 | ociger smoke row
(1 row)

pg_relation_filepath(public.demo_rows)=base/16384/16385
host_path=/Users/neoxr/git_sporaxis-com/oci-germination/.artifacts/ociger-core-pg17-smoke/pgdata/base/16384/16385
```

That is the current proof that data written by the container lands in the host-mounted `PGDATA`.

## Feature Matrix

### Artifact Matrix

| Artifact | Type | Runnable | Platforms | Includes | Source Strategy | Size | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `core-pg17-min` | OCI image | yes | `linux/amd64`, `linux/arm64` | PostgreSQL 17 | extract runtime from `postgres:17-bookworm` into distroless final image | `139.5 MiB` uncompressed, `53.1 MiB` compressed local archive | active |
| `core-pg17-debug` | OCI image | yes | `linux/amd64`, `linux/arm64` | PostgreSQL 17 | larger debug/control image for inspection and diffing | TBD | planned |
| `bundle-pg17-pgrdf` | OCI image | yes | `linux/amd64`, `linux/arm64` | PostgreSQL 17 + `pgRDF` | pull upstream `pgRDF` OCI artifact during build | TBD | planned |
| `bundle-pg17-pgrdf-pgck` | OCI image | yes | `linux/amd64`, `linux/arm64` | PostgreSQL 17 + `pgRDF` + `pgCK` | pull upstream `pgRDF` and `pgCK` OCI artifacts during build | TBD | critical goal |
| `bundle-index` | OCI index / artifact | no | n/a | bundle metadata only | future OCI root manifest for materialization | TBD | planned |

### Combination Matrix

| Combination | Runnable Image | Non-Runnable Artifact | Planned Here | Notes |
| --- | --- | --- | --- | --- |
| `PG17` | yes | later | yes | current core layer |
| `PG17 + pgRDF` | yes | yes | yes | next assembly step after core |
| `PG17 + pgCK` | no | no | no | `pgCK` is not treated as a standalone replacement for `pgRDF` |
| `PG17 + pgRDF + pgCK` | yes | yes | yes | main bundle goal |
| `pgRDF` only | no | yes | upstream-owned | consumed here, not built here |
| `pgCK` only | no | yes | upstream-owned | consumed here, not built here |
| `Bundle manifest only` | no | yes | yes | future ORAS/OCI descriptor |

### Behavior Matrix

| Artifact | First Boot | Auth Default | Persistence | Local Smoke Path | Notes |
| --- | --- | --- | --- | --- | --- |
| `core-pg17-min` | runs `initdb` when `PGDATA` is empty | `trust` for zero-config local startup | bind mount verified | `bash scripts/smoke-core-pg17.sh` | local/dev only |
| `bundle-pg17-pgrdf` | same pattern | TBD | planned | TBD | extension creation to be added |
| `bundle-pg17-pgrdf-pgck` | same pattern | TBD | planned | TBD | must be ready-to-go on first launch |

## Security Posture

Current `core-pg17-min` defaults are intentionally optimized for frictionless local startup:

- `listen_addresses='*'`
- `host all all all trust`
- no password required for the default smoke path

That is acceptable for local isolated testing and unacceptable for exposed or shared networks. Treat the current image as a local/dev artifact until passworded startup and stricter auth defaults are added.

## Repository Layout

Key files:

- `bundles/core-pg17/bundle.yaml`: source of truth for the core bundle
- `bundles/core-pg17/Dockerfile`: generated runtime build
- `bundles/core-pg17/docker-bake.hcl`: generated Bake config
- `cmd/ociger-gen/main.go`: generator entrypoint
- `cmd/ociger-pg-launcher/main.go`: first-boot launcher
- `scripts/build-core-pg17.sh`: native-platform local build
- `scripts/smoke-core-pg17.sh`: contained smoke test with host persistence proof
- `.github/workflows/core-pg17-release.yml`: CI and GHCR publish workflow

## Local Development

Generate the bundle outputs:

```bash
bash scripts/generate.sh
```

Build the local image for the native Docker architecture:

```bash
bash scripts/build-core-pg17.sh
```

Run the full persistence smoke test:

```bash
bash scripts/smoke-core-pg17.sh
```

If you need to force a specific local build platform:

```bash
OCI_GER_PLATFORM=linux/amd64 bash scripts/build-core-pg17.sh
```

The local workflow is intentionally contained:

- container name: `ociger-core-pg17-smoke`
- network name: `ociger-core-pg17-net`
- data path: `.artifacts/ociger-core-pg17-smoke/pgdata`

Cleanup logic only removes those exact `ociger-` resources when the ownership label matches this repo's smoke test.

## Release Flow

The release workflow is tag-driven.

Workflow trigger:

- push a tag matching `core-pg17-v*`

What the workflow does:

1. runs `go test ./...`
2. regenerates `Dockerfile` and `docker-bake.hcl`
3. verifies generated files are committed
4. builds the native smoke image
5. runs the smoke test
6. publishes a multi-arch GHCR image for `linux/amd64` and `linux/arm64`
7. tags the package with both the version tag and `latest`
8. creates or updates the GitHub release for that tag

Manual release example:

```bash
git push origin main
git tag core-pg17-vX.Y.Z
git push origin core-pg17-vX.Y.Z
```

## Why No Compose

This repo deliberately does not use `compose.yaml` as the assembly primitive.

The goals here are:

- normal multi-stage Dockerfiles
- generator-driven bundle configuration
- OCI-native publication to GHCR
- future OCI artifact consumption for `pgRDF` and `pgCK`

Compose may still be useful at the application layer later, but it is not the core bundle definition surface here.

## ORAS and OCI Artifact Roadmap

The longer-term manifest story is:

1. upstream projects publish their own extension artifacts
2. this repo pulls those OCI artifacts during bundle builds
3. this repo publishes runnable bundle images
4. this repo later publishes a higher-level OCI bundle descriptor or image index that points at the runnable and non-runnable parts together

That means:

- no custom low-level layer assembler in this repo
- standard Docker build mechanics for image construction
- ORAS/OCI artifact handling for extension inputs and future bundle descriptors

## Upstream References

- `pgRDF` repository: `https://github.com/styk-tv/pgRDF`
- `pgRDF` install docs: `https://pgrdf.styk.tv/v0.5/operations/install`
- `pgCK` repository: `https://github.com/styk-tv/pgCK`

## Near-Term Roadmap

1. publish and verify `core-pg17-min` from GHCR
2. consume upstream `pgRDF` OCI artifacts in a generated bundle build
3. consume upstream `pgCK` OCI artifacts in the same generated bundle build
4. make the triple bundle ready-to-go on first launch
5. add a non-runnable OCI bundle descriptor for materialization workflows
