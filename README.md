# OCI Germination

`oci-germination` publishes small OCI PostgreSQL bundles without rebuilding upstream extension projects in this repo.

Current public releases:

- `core-pg17` — minimal PostgreSQL 17 runtime
- `pg17-pgrdf` — PostgreSQL 17 + `pgRDF 0.5.1`

Next target:

- `pg17-pgrdf-pgck` — PostgreSQL 17 + `pgRDF` + `pgCK`

## Published Images

All published images are multi-arch manifest lists for `linux/amd64` and `linux/arm64`.

| Bundle | OCI image | GitHub release | Platforms | Verified behavior |
| --- | --- | --- | --- | --- |
| `core-pg17` | `ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2` | `core-pg17-v0.1.2` | `amd64`, `arm64` | boots, initializes `PGDATA`, creates DB/table/row, proves relation file on host bind mount |
| `pg17-pgrdf` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.1` | `pg17-pgrdf-v0.1.1` | `amd64`, `arm64` | `CREATE EXTENSION pgrdf`; `pg_available_extensions.default_version=0.5.1`; `pg_extension.extversion=0.5.1`; `pgrdf.version()=0.5.1` |

Pinned manifest digests:

- `core-pg17-v0.1.2` → `sha256:bd11e0bc6b39577e1e8e946dc28fcf3a7cea24524472a97c68c408a80ef4e3ac`
- `pg17-pgrdf:v0.1.1` → `sha256:31e2bdbb34d2ddf3fec609eb748ef9bec0707a019adb386d0d095463abb20d61`

## Bundle Chain

| Stage | Inputs | Output | Status | Notes |
| --- | --- | --- | --- | --- |
| `core-pg17` | `postgres:17-bookworm` runtime files + distroless final image | `ociger-core-pg17-min` | released | minimal PostgreSQL 17 runtime, no extensions |
| `pg17-pgrdf` | same minimal PG17 runtime shape + upstream `pgRDF 0.5.1` release artifact | `ociger-pg17-pgrdf` | released | current working extension bundle |
| `pg17-pgrdf-pgck` | PG17 runtime + `pgRDF` + `pgCK` | pending | next | target triple bundle |

`pg17-pgrdf` is linked to the same minimal PG17 runtime shape as `core-pg17`, then adds the `pgRDF` extension files during the image build. The next bundle adds `pgCK` on top of that sequence.

## One-Line Launch

Run the minimal core image:

```bash
docker run --rm -d \
  --name ociger-core-pg17 \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-core-pg17-data:/var/lib/postgresql/data" \
  -p 15432:5432 \
  ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2
```

Then connect:

```bash
psql -h 127.0.0.1 -p 15432 -U postgres -d postgres
```

Run the `pgRDF` bundle:

```bash
docker run --rm -d \
  --name ociger-pg17-pgrdf \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-pg17-pgrdf-data:/var/lib/postgresql/data" \
  -p 15433:5432 \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.1
```

Then verify the extension surface:

```bash
psql -h 127.0.0.1 -p 15433 -U postgres -d postgres \
  -c 'CREATE EXTENSION pgrdf; SELECT pgrdf.version();'
```

## Verification

Core proof:

```bash
bash scripts/smoke-core-pg17.sh
bash scripts/smoke-core-pg17.sh ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2
```

`pgRDF` proof:

```bash
bash scripts/smoke-pg17-pgrdf.sh
bash scripts/smoke-pg17-pgrdf.sh ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.1
```

Expected `pgRDF` smoke output:

```text
CREATE EXTENSION
pg_available_extensions.default_version=0.5.1
pg_extension.extversion=0.5.1
pgrdf.version()=0.5.1
```

Measured core footprint:

- uncompressed image size: `146277807` bytes (`139.5 MiB`)
- compressed local archive size: `55630530` bytes (`53.1 MiB`)

No bundled `pgRDF` size is claimed here yet because it has not been measured and recorded the same way.

## Local Development

Generate bundle outputs:

```bash
bash scripts/generate.sh
```

Build locally:

```bash
bash scripts/build-core-pg17.sh
bash scripts/build-pg17-pgrdf.sh
```

If you need to force a local architecture:

```bash
OCI_GER_PLATFORM=linux/amd64 bash scripts/build-core-pg17.sh
OCI_GER_PLATFORM=linux/amd64 bash scripts/build-pg17-pgrdf.sh
```

Local smoke resources stay contained to `ociger-`-prefixed names and `.artifacts/ociger-*` paths.

## Repository Layout

- `bundles/core-pg17/` — core PostgreSQL bundle spec and generated build files
- `bundles/bundle-pg17-pgrdf/` — `pgRDF` bundle spec and generated build files
- `cmd/ociger-gen/` — bundle generator
- `cmd/ociger-pg-launcher/` — minimal first-boot PostgreSQL launcher
- `scripts/build-*.sh` — local native-arch image builds
- `scripts/smoke-*.sh` — contained local and public-image smoke tests
- `.github/workflows/` — release verification and GHCR publish workflows

## Release

Release tags:

- `core-pg17-vX.Y.Z`
- `pg17-pgrdf-vX.Y.Z`

Current workflows:

- [core-pg17-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/core-pg17-release.yml)
- [pg17-pgrdf-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/pg17-pgrdf-release.yml)

Manual release example:

```bash
git push origin main
git tag core-pg17-vX.Y.Z
git tag pg17-pgrdf-vX.Y.Z
git push origin core-pg17-vX.Y.Z
git push origin pg17-pgrdf-vX.Y.Z
```

## Defaults

Current images are optimized for local startup and smoke verification, not exposed production use:

- `listen_addresses='*'`
- `host all all all trust`
- no password required for the default smoke path

Treat the current images as local/dev artifacts until stricter auth defaults are introduced.

## Upstream Inputs

- `pgRDF` repository: `https://github.com/styk-tv/pgRDF`
- `pgRDF` install docs: `https://pgrdf.styk.tv/v0.5/operations/install`
- `pgCK` repository: `https://github.com/styk-tv/pgCK`
