# PG17 + pgRDF + pgCK First-Slice Design

Date: 2026-05-26
Status: Approved design
Scope: first runnable triple-bundle slice for `pg17-pgrdf-pgck`, limited to extension install and version verification

## Problem Statement

The repository already publishes:

- `core-pg17` as a minimal PostgreSQL 17 runtime
- `pg17-pgrdf` as PostgreSQL 17 with `pgRDF 0.5.1`

The next required step is the first public triple bundle:

- PostgreSQL 17
- `pgRDF 0.5.1`
- `pgCK 0.1.2`

This repository must not build `pgRDF` or `pgCK` from source. It must consume their already-published upstream release artifacts and OCI artifacts, assemble a runnable image, and prove that the extension SQL version surfaces are correct.

## Goals

- Publish a runnable public `pg17-pgrdf-pgck` image for `linux/amd64` and `linux/arm64`.
- Keep the bundle build shape consistent with the existing generator-driven approach.
- Install `pgCK` from its published OCI artifact path:
  - `ghcr.io/styk-tv/pgck:0.1.2-pg17-<arch>`
- Keep `pgRDF` on the currently proven `0.5.1` input path.
- Start PostgreSQL with `pgck` preloaded by default.
- Prove that both extensions create successfully and report the expected versions.
- Document the new bundle variant and release it publicly.

## Non-Goals

- Running a `ckp.*` workflow in this first slice.
- Validating NATS, WSS, or bgworker runtime behavior beyond preload and extension creation.
- Changing the current `core-pg17` or `pg17-pgrdf` release contracts.
- Replacing the current `pgRDF` fetch path in this slice.

## Upstream Inputs

### `pgRDF`

- version: `0.5.1`
- current repo contract:
  - GitHub release tarball
  - `pgrdf.version()` returns `0.5.1`
  - `pgrdf.control` `default_version = '0.5.1'`

### `pgCK`

- version: `0.1.2`
- public OCI artifact:
  - `ghcr.io/styk-tv/pgck:0.1.2-pg17-amd64`
  - `ghcr.io/styk-tv/pgck:0.1.2-pg17-arm64`
- native SQL version surface:
  - `SELECT pgck_version();` returns `pgck 0.1.2 (rc3)`
- control file:
  - `default_version = '0.1.2'`
  - `requires = 'pgrdf, pgcrypto'`
- preload requirement:
  - `shared_preload_libraries = 'pgck'`

## Architecture

The new bundle is a sibling to the current bundles, not a rebuild of upstream extension code.

It will be generated into:

```text
bundles/bundle-pg17-pgrdf-pgck/
  bundle.yaml
  Dockerfile
  docker-bake.hcl
```

The build shape is:

1. Build the existing tiny Go launcher.
2. Fetch `pgRDF 0.5.1` from the proven upstream release tarball path.
3. Pull `pgCK 0.1.2` from its published upstream OCI artifact using ORAS.
4. Copy PostgreSQL 17 runtime files from `postgres:17-bookworm`.
5. Copy `pgrdf.so`, `pgck.so`, and both extensions' control and SQL files into the final runtime tree.
6. Set the image default so PostgreSQL starts with `shared_preload_libraries='pgck'`.

The final image remains a runnable distroless-like image with no package manager in the runtime layer.

## Startup Contract

The first-slice triple bundle must start PostgreSQL with `pgck` preloaded by default on every boot.

This is an image default, not a manual runtime instruction.

The simplest acceptable implementation is:

- set an image environment variable declaring the preload list
- have the launcher append `-c shared_preload_libraries=<value>` to the `postgres` exec args

This approach is preferred over writing the setting only into `postgresql.conf`, because it keeps the default active on every container boot even when `PGDATA` already exists.

## Smoke-Test Contract

The first triple-bundle smoke test is intentionally narrow.

It must:

1. boot a fresh cluster
2. assert `pgrdf` is available in `pg_available_extensions`
3. assert `pgck` is available in `pg_available_extensions`
4. run:
   - `CREATE EXTENSION pgrdf;`
   - `CREATE EXTENSION pgck CASCADE;`
5. assert exact installed versions from `pg_extension`
6. assert exact native version surfaces

Required SQL checks:

```sql
SELECT default_version
FROM pg_available_extensions
WHERE name = 'pgrdf';

SELECT default_version
FROM pg_available_extensions
WHERE name = 'pgck';

SELECT extname, extversion
FROM pg_extension
WHERE extname IN ('pgrdf', 'pgck')
ORDER BY extname;

SELECT pgrdf.version();
SELECT pgck_version();
```

Expected values:

- `pgrdf` available default version: `0.5.1`
- `pgck` available default version: `0.1.2`
- `pgrdf` installed extversion: `0.5.1`
- `pgck` installed extversion: `0.1.2`
- `pgrdf.version()`: `0.5.1`
- `pgck_version()`: `pgck 0.1.2 (rc3)`

Failure classes must stay explicit:

- `artifact-missing`
- `not-installed`
- `wrong-version`
- `native-version-empty`
- `native-version-mismatch`

## Local Resource Rules

The new bundle must keep the existing containment rules:

- container names start with `ociger-`
- networks start with `ociger-`
- artifact directories stay under `.artifacts/ociger-*`
- no touching unrelated Colima or Docker workloads

Recommended smoke names:

- image: `ociger-pg17-pgrdf-pgck:local`
- container: `ociger-pg17-pgrdf-pgck-smoke`
- network: `ociger-pg17-pgrdf-pgck-net`
- data dir: `.artifacts/ociger-pg17-pgrdf-pgck-smoke/pgdata`

## CI and Release

The repository should add a third release workflow matching the existing style:

- verify on `pull_request`, `push`, `workflow_dispatch`, and `schedule`
- publish on tags matching:
  - `pg17-pgrdf-pgck-v*`

The release image target should be:

- `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck`

The first release tag should be:

- `pg17-pgrdf-pgck-v0.1.1`

The publish job must:

1. regenerate bundle outputs
2. build and smoke-test the native image
3. publish a multi-arch image for `linux/amd64,linux/arm64`
4. tag the image as:
   - `v0.1.1`
   - `latest`
5. attempt GHCR package publicization without making the whole release fail if the visibility API path is skipped or unavailable

## Documentation Changes

The README must add the new working variant after release.

It should document:

- the new public image
- the release tag
- supported architectures
- how the triple bundle links the PG17 runtime, `pgRDF`, and `pgCK`
- one-line launch command
- the exact version verification output

The README tables must stay compact.

## Risks and Tradeoffs

### `pgRDF` source path mismatch

`pgRDF` is still on the currently proven tarball-based input path in this repo, while `pgCK` is already consumed as a public OCI artifact.

That is acceptable for this slice because the user priority is a working public triple bundle, not input-path purity.

### `pgCK` preload requirement

`pgCK` needs `shared_preload_libraries='pgck'`.

If that is not applied on every boot, extension behavior may differ between first boot and restart. That is why the preload default belongs in the launcher exec path.

### Visibility API instability

The GHCR visibility step has already shown `404` behavior even when package pulls succeed. Release workflows must not fail the whole publish after the image push solely because that visibility call is skipped or unavailable.

## Acceptance Criteria

This slice is complete when all of the following are true:

1. `go test ./...` passes
2. the generated triple-bundle Dockerfile and bake file are committed
3. the local build succeeds for the native Docker architecture
4. the local triple-bundle smoke test passes
5. the tag `pg17-pgrdf-pgck-v0.1.1` publishes `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1`
6. anonymous pull of that public image succeeds
7. the smoke test passes against the public image
8. the README documents the new variant accurately
