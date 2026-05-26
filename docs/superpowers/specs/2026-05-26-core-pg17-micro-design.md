# Core PG17 Micro Bundle Design

Date: 2026-05-26
Status: Approved design
Scope: add a new additional `core-pg17-micro` OCI bundle with a smaller runtime footprint than the current `core-pg17` line

## Problem Statement

The repository already publishes a working `core-pg17` image and uses that runtime shape as the foundation for the released extension bundles.

That stable line must stay intact.

The new task is to add a second core PostgreSQL bundle that is explicitly optimized for footprint:

- still runnable
- still multi-arch for `linux/amd64` and `linux/arm64`
- still using standard OCI image builds in this repo
- still compatible with the existing launcher and bind-mounted `PGDATA` smoke proof
- but materially smaller than the current `core-pg17` image

This is an additional bundle possibility, not a layer replacement and not a rewrite of the released `core-pg17`, `pg17-pgrdf`, or `pg17-pgrdf-pgck` lines.

## Goals

- Add a new runnable image:
  - `ghcr.io/sporaxis-com/ociger-core-pg17-micro`
- Keep the same PostgreSQL major version:
  - `17`
- Preserve the same core smoke contract:
  - boot
  - initialize `PGDATA`
  - create database
  - create table
  - insert row
  - query row
  - prove relation file on the host bind mount
- Keep the filesystem layout compatible with later extension-oriented bundle work:
  - `/usr/lib/postgresql/17/...`
  - `/usr/share/postgresql/17/...`
- Reduce both the uncompressed and compressed image sizes relative to the current `core-pg17` release.
- Document the new bundle as a separate variant in the README matrix and bundle chain.

## Non-Goals

- Replacing the existing `core-pg17` release line.
- Releasing a `musl` or source-built PostgreSQL in this slice.
- Promising a `20-35MB` uncompressed image from the Debian-linked PostgreSQL input path.
- Proving `pgRDF` or `pgCK` installation inside the micro image in this slice.
- Building `pgrdf` or `pgck` here.

## Baseline Findings

Current measured `core-pg17` footprint:

- uncompressed image size: `146277807` bytes (`139.5 MiB`)
- compressed local archive size: `55630530` bytes (`53.1 MiB`)

Measured contributors inside the current local `core-pg17` rootfs:

- `/usr/lib/postgresql/17/bin` about `15M`
- `/usr/lib/postgresql/17/lib` about `32M`
- copied runtime libraries under `/lib` and `/usr/lib` about `85M` combined in the extracted rootfs
- `gcr.io/distroless/base-debian12:latest` image size about `31367138` bytes (`29.9 MiB`)

The single largest confirmed runtime library in the current Debian-linked stack is:

- `libicudata.so.72` about `30M` on local `linux/arm64`

That means the micro lane can cut meaningful weight, but it should not claim extreme embedded sizes while still depending on the stock `postgres:17-bookworm` runtime binaries.

## Architecture

The new bundle is a sibling of the existing bundles:

```text
bundles/core-pg17/
bundles/core-pg17-micro/
bundles/bundle-pg17-pgrdf/
bundles/bundle-pg17-pgrdf-pgck/
```

It will generate into:

```text
bundles/core-pg17-micro/
  bundle.yaml
  Dockerfile
  docker-bake.hcl
```

The image contract remains the same as `core-pg17`:

- same launcher entrypoint
- same `PGDATA` default
- same first-boot `initdb`
- same local smoke pattern

The size strategy changes in two ways:

1. the PostgreSQL runtime tree is copied selectively instead of wholesale
2. the final image uses `scratch`, not distroless, so the runtime contains only files explicitly materialized by the build

The micro variant should remain glibc-based so later slim extension bundles can still target the same ABI and filesystem layout if we decide to build them from the same pattern.

## Build Strategy

The micro Dockerfile should follow this shape:

1. Build the existing static Go launcher.
2. Use `postgres:17-bookworm` as the source stage.
3. Materialize a minimal `/out` tree containing only the files the runtime actually needs.
4. Use `FROM scratch` for the final stage.
5. Copy `/out` and the static launcher into the final image.

The default implementation should not silently fall back to a heavier final base image.

If a `scratch` runtime fails verification, that should be treated as a design break that needs a revision, not as a reason to quietly revert to the current distroless pattern.

## File Selection Strategy

### Binaries To Keep

Keep only:

- `/usr/lib/postgresql/17/bin/postgres`
- `/usr/lib/postgresql/17/bin/initdb`

Do not ship:

- `psql`
- `pg_dump`
- `pg_restore`
- `pg_ctl`
- `pg_basebackup`
- `pgbench`
- any of the other maintenance or client binaries

The image runtime contract does not require those tools because the smoke path already uses helper containers for readiness checks and SQL execution.

### PostgreSQL Libraries To Keep

Keep only the minimum PostgreSQL private library files required for core runtime behavior.

The default keep set is:

- `plpgsql.so`

The micro image should not carry the full `/usr/lib/postgresql/17/lib` tree.

If verification proves another PostgreSQL private library is required for boot or `initdb`, it may be added explicitly, but only with a direct reason captured in the Dockerfile comments.

### Shared System Libraries To Keep

Copy only the exact dynamic-library dependencies reported by `ldd` for:

- `postgres`
- `initdb`
- `plpgsql.so`

Do not copy broad library directories.

This keeps the runtime tied to the actual dynamic-link closure instead of the current broader Debian rootfs shape.

### Share Files To Keep

Keep the minimal PostgreSQL share content required for cluster initialization and default runtime behavior:

- `postgres.bki`
- `information_schema.sql`
- `system_functions.sql`
- `system_views.sql`
- `system_constraints.sql`
- `tsearch_data/`
- `timezonesets/`
- config samples:
  - `postgresql.conf.sample`
  - `pg_hba.conf.sample`
  - `pg_ident.conf.sample`
- extension files required for default bootstrap behavior:
  - `extension/plpgsql.control`
  - matching `plpgsql--*.sql`

Do not ship:

- `man/`
- docs
- `psqlrc.sample`
- `pg_service.conf.sample`
- unrelated extension SQL/control files

### Identity Files

Do not copy full Debian identity files into the micro image.

Instead, materialize minimal runtime files for:

- `/etc/passwd`
- `/etc/group`

They only need the baseline entries required for the launcher to operate correctly and to append a bind-mount owner mapping when needed.

## Runtime Behavior

`core-pg17-micro` keeps the current launcher behavior:

- `PGDATA=/var/lib/postgresql/data`
- first boot runs `initdb`
- bootstrap uses:
  - `--username=postgres`
  - `--auth=trust`
  - `--locale=C`
  - `--encoding=UTF8`
- container listens on `0.0.0.0`
- default smoke path remains trust-auth local/dev behavior

This slice does not add `pgrdf`, `pgck`, or other extension files.

The micro variant is focused on a smaller runnable PostgreSQL core with the same boot and persistence proof.

## Verification Contract

The micro bundle must pass the same functional proof as the current core image.

Required verification steps:

1. build the native-arch image locally
2. boot the image in an `ociger-` prefixed smoke environment
3. create a database
4. create a table
5. insert a row
6. query the row back
7. prove the table relation file exists under the bind-mounted `PGDATA`

The repo should add bundle-specific scripts matching the current pattern:

- `scripts/build-core-pg17-micro.sh`
- `scripts/smoke-core-pg17-micro.sh`

The new smoke resources must stay isolated:

- image: `ociger-core-pg17-micro:local`
- container: `ociger-core-pg17-micro-smoke`
- network: `ociger-core-pg17-micro-net`
- data dir: `.artifacts/ociger-core-pg17-micro-smoke/pgdata`

## Release and CI

The repository should add a dedicated workflow:

- `.github/workflows/core-pg17-micro-release.yml`

It should follow the existing release style:

- verify on `pull_request`, `push`, `workflow_dispatch`, and `schedule`
- publish on tags matching:
  - `core-pg17-micro-v*`

Release image:

- `ghcr.io/sporaxis-com/ociger-core-pg17-micro`

First planned release tag:

- `core-pg17-micro-v0.1.0`

The release workflow must:

1. run `go test ./...`
2. regenerate bundle outputs and verify they are committed
3. build the native-arch micro image
4. run the micro smoke test locally in CI
5. publish a multi-arch image for `linux/amd64,linux/arm64`
6. tag the image as:
   - `v0.1.0`
   - `latest`
7. attempt GHCR visibility/publicization the same way as the existing release workflows without failing the publish after a successful image push

## Documentation Changes

The README must add `core-pg17-micro` as a separate released variant, not as a replacement for `core-pg17`.

It should document:

- OCI image name
- GitHub release tag
- supported architectures
- measured compressed and uncompressed sizes
- what was removed to make it smaller
- the same one-line launch flow used for the other runnable bundles

The published image table should remain compact.

## Risks and Tradeoffs

### ICU Floor

The stock Debian-linked PostgreSQL runtime depends on ICU libraries, and `libicudata.so.72` alone is a major footprint contributor.

That puts a hard floor under this bundle family.

The micro variant should aim for a materially smaller image, not a fantasy size target that the current upstream binary choice cannot support.

### Share-File Fragility

`initdb` depends on PostgreSQL share assets that are not all obvious from dynamic-link inspection.

That is why the design keeps a conservative but still selective share-file set instead of trying to overfit the minimal file list on the first pass.

### Private Library Pruning

Dropping most of `/usr/lib/postgresql/17/lib` is safe only if the retained set is verified against the real smoke contract.

The design therefore starts with the smallest explicit keep set and allows additions only when verification proves they are required.

### Separate Variant Complexity

Adding a second core image increases the matrix surface.

That is acceptable because the user explicitly wants multiple OCI bundle possibilities rather than one fixed runtime line.

## Acceptance Criteria

This slice is complete when all of the following are true:

1. the repo contains `bundles/core-pg17-micro/` with committed generated outputs
2. local `go test ./...` passes
3. the local native-arch `core-pg17-micro` build succeeds
4. the local `core-pg17-micro` smoke test passes
5. both measured sizes are smaller than the current released `core-pg17` measurements
6. the tag `core-pg17-micro-v0.1.0` publishes a public multi-arch image
7. anonymous pull of the public image succeeds
8. the smoke test passes against the public image
9. the README documents `core-pg17-micro` accurately as an additional bundle variant
