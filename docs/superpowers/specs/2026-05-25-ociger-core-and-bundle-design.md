# OCI Germination Core and Bundle Design

Date: 2026-05-25
Status: Approved design
Scope: first deliverable is `core-pg17`; critical release goal remains the public triple bundle with PostgreSQL 17, `pgrdf`, and `pgck`

## Problem Statement

This repository exists to publish very small OCI-delivered PostgreSQL bundles without rebuilding upstream extension projects here.

The immediate problem is to prove that we can ship an extremely small embedded PostgreSQL 17 runtime that:

- runs on `linux/amd64` and `linux/arm64`
- is public and easy to launch
- avoids local collisions in Colima and Docker
- creates durable data in a bind-mounted data directory
- makes the physical storage location easy to prove and document

The follow-on problem is to assemble a public triple bundle that combines:

- embedded PostgreSQL 17
- upstream `pgrdf` OCI extension artifact
- upstream `pgck` OCI extension artifact

This repository must not build `pgrdf` or `pgck` from source. It consumes their already-published OCI artifacts and assembles higher-level runnable bundles around them.

## Goals

- Publish a public `core-pg17` image that is as close to distroless as PostgreSQL can practically support.
- Keep the runtime in the tens of megabytes if possible, and record actual measured sizes instead of guessing.
- Use normal Dockerfiles and BuildKit, not a custom layer assembly engine.
- Introduce a small checked-in declarative bundle spec that generates Docker build inputs and CI config.
- Keep the path open to a future single OCI bundle descriptor that references all parts of a release.
- Publish multi-architecture images for `linux/amd64` and `linux/arm64`.
- Produce strong documentation from the start: feature matrix, release matrix, size matrix, and storage proof.

## Non-Goals

- Building PostgreSQL from source in this repository.
- Building `pgrdf` or `pgck` from source in this repository.
- Using `compose.yaml` as a published launch surface.
- Modifying the Colima VM, changing Colima profiles, or touching existing pods or workloads.
- Implementing a custom OCI assembler when Docker and BuildKit already provide the image assembly mechanics.

## Design Principles

1. Use Dockerfiles like normal people. Generated Dockerfiles are acceptable; custom binary layer assembly is not.
2. Keep one small declarative source of truth per bundle and generate the repetitive build files from it.
3. Prefer shell-free or near-shell-free final images.
4. Keep debug surfaces separate from release surfaces.
5. Treat `pgrdf` and `pgck` as external OCI-delivered inputs, not code that belongs to this repository.
6. Keep all local resources prefixed with `ociger-` to avoid collisions.
7. Measure everything that matters: image size, compressed size, startup behavior, and on-disk data paths.

## Architecture Overview

The repository will define bundles declaratively and generate standard Docker build files from those specs.

The primary build model is:

1. A bundle spec declares bundle metadata, upstream inputs, platforms, launch defaults, and documentation metadata.
2. A small generator renders:
   - `Dockerfile`
   - `docker-bake.hcl`
   - GitHub Actions workflow fragments or generated workflows
   - README matrix fragments or matrix source data
3. Docker BuildKit performs the actual image assembly.
4. GitHub Actions publishes the built images to GHCR.
5. Later, a single OCI bundle descriptor can be published as an OCI image index that points at the released runnable images and related artifacts.

This keeps the mechanics conventional while removing duplication across Dockerfiles, Bake targets, CI jobs, and docs.

## Why A Generator Instead Of Compose

Compose solves a different problem: defining and running multi-container applications. This repository needs a repeatable build and publishing model for OCI bundles, not a Compose application package.

The chosen model is:

- bundle spec as the source of truth
- generated Dockerfile and Bake config for building
- no checked-in `compose.yaml`
- optional future OCI bundle descriptor for release aggregation

## OCI Model

Two OCI concepts matter here:

- OCI image manifests for runnable images
- OCI image indexes for higher-level bundle descriptors and multi-architecture entry points

Immediate releases will publish runnable OCI images.

Later releases may also publish a non-runnable bundle descriptor as an OCI image index that references:

- `core-pg17` image manifests
- `bundle-pg17-pgrdf` image manifests
- `triple-pg17-pgrdf-pgck` image manifests
- extension source descriptors
- documentation and metadata artifacts

The index is the right future root object because it is the higher-level OCI manifest form for describing a set of manifests.

## Repository Shape

The repository should grow into this structure:

```text
bundles/
  core-pg17/
    bundle.yaml
    Dockerfile
    docker-bake.hcl
  bundle-pg17-pgrdf/
    bundle.yaml
    Dockerfile
    docker-bake.hcl
  triple-pg17-pgrdf-pgck/
    bundle.yaml
    Dockerfile
    docker-bake.hcl

cmd/
  ociger-gen/
    main.go

docs/
  superpowers/
    specs/
    plans/
  matrices/
    bundles.csv
    bundles.md

.github/
  workflows/
```

The generator implementation should be in Go so it is easy to keep small, static, and portable.

## Bundle Spec Contract

Each `bundle.yaml` should define:

- bundle name
- human description
- runnable or non-runnable classification
- target platforms
- base image input
- OCI artifact inputs
- output image name and tags
- launch defaults
- local smoke-test metadata
- documentation metadata for the feature matrix

The generator should treat the bundle spec as authoritative and render the Dockerfile, Bake definition, and CI publishing definition from it.

Generated files should be checked in. This keeps the build surface inspectable and avoids making CI depend on hidden generated state.

## First Deliverable: `core-pg17`

The first deliverable is a runnable public image that provides embedded PostgreSQL 17 with no extensions.

Release intent:

- public
- runnable
- multi-arch
- minimum practical size
- shell-free or near-shell-free final image
- package-manager-free final image

This first deliverable is the proving ground for everything that comes after it.

### `core-pg17` Variants

Two variants are required:

1. `core-pg17-min`
   - release target
   - smallest practical runnable image
   - no debug tooling

2. `core-pg17-debug`
   - inspection target
   - larger image for diagnostics and comparison
   - not the release target

The debug variant exists to make the minimal variant easier to understand and verify without bloating the main runtime.

## `core-pg17` Runtime Strategy

The release image will be assembled from upstream PostgreSQL container/runtime assets, not by compiling PostgreSQL in this repository.

The expected build shape is:

1. Builder or extractor stages pull an upstream PostgreSQL 17 runtime image.
2. Those stages copy only the required PostgreSQL binaries, shared libraries, support files, user/group metadata, and data-directory defaults into the final image.
3. The final image contains a tiny first-boot launcher that:
   - initializes `PGDATA` if empty
   - starts PostgreSQL
   - stays as the container entrypoint process

The final image should avoid carrying a general-purpose shell unless the implementation proves that removing it is not worth the size or complexity tradeoff.

Pure `scratch` may or may not be achievable. The design target is "distroless-like minimum practical runtime", not a promise of zero dynamic dependencies.

## Local Collision Rules

All local resources created by this repository must use the `ociger-` prefix.

Required prefixes:

- container names: `ociger-*`
- networks: `ociger-*`
- local artifact directories: `.artifacts/ociger-*`
- temporary build containers: `ociger-*`

The implementation must not:

- edit Colima configuration
- restart Colima
- switch Docker contexts
- touch the `colima-k8s` profile
- touch unrelated local containers or pods

The default local smoke test should not publish a host port. Instead, it should use a dedicated Docker network and an ephemeral client container to avoid collisions with anything already using port `5432`.

## First Smoke-Test Contract

The first smoke test must prove all of the following:

1. The `core-pg17-min` image starts successfully.
2. A fresh cluster is initialized in a bind-mounted `PGDATA`.
3. A new database can be created.
4. A table can be created.
5. A row can be inserted.
6. The row can be queried back.
7. The physical relation path can be shown.
8. The corresponding file can be located on the host filesystem.

Recommended smoke-test naming:

- network: `ociger-core-pg17-net`
- server container: `ociger-core-pg17-smoke`
- host data path: `.artifacts/ociger-core-pg17-smoke/pgdata`
- demo database: `ociger_demo`
- demo table: `public.demo_rows`

## Data Storage Proof

The documentation must show the data path at two levels:

1. PostgreSQL-reported relative relation path
2. host filesystem path under the bind-mounted `PGDATA`

The proof sequence should look like this:

1. create database `ociger_demo`
2. create table `public.demo_rows`
3. insert at least one row
4. run:

```sql
SELECT pg_relation_filepath('public.demo_rows'::regclass);
```

5. map that relative path onto:

```text
.artifacts/ociger-core-pg17-smoke/pgdata/<returned-relative-path>
```

The README must then show both:

- the SQL result from `pg_relation_filepath(...)`
- the matching host path where the table file lives

This is important because the first layer is not "real" until we can show exactly where the bytes are landing.

## Extension Bundle Strategy

This repository will not build `pgrdf` or `pgck`.

Instead, later bundle builds will:

1. resolve the required upstream OCI artifacts by digest or explicit version tag
2. materialize those artifacts into a local build context
3. copy the resulting extension files into the runnable PostgreSQL image
4. initialize the database so the extensions are ready to use on first boot

This preserves ownership boundaries:

- upstream repos build and publish extension artifacts
- this repo assembles runnable bundles that consume those artifacts

## Why ORAS Is The Right Artifact Layer

ORAS is the correct transport and artifact tool here because the upstream extension outputs are OCI artifacts, not ordinary runnable container images.

This repository should use ORAS for:

- pulling OCI extension artifacts
- inspecting descriptors and annotations
- materializing artifact contents into build inputs
- publishing future non-runnable bundle descriptors

Docker and BuildKit remain responsible for image assembly.

## `pgrdf` And `pgck` Build Integration

The desired long-term bundle build is:

- `core-pg17` base prepared by this repository
- `pgrdf` artifact pulled and unpacked during build preparation
- `pgck` artifact pulled and unpacked during build preparation
- final image built with a standard Dockerfile

The bundle generator should therefore support "artifact materialization steps" as pre-build inputs, but the Dockerfile itself should stay conventional.

This design deliberately avoids inventing a custom OCI layer merger.

## Launch Contract For The Triple Bundle

The critical release goal is a public triple bundle where a single standard Docker command launches PostgreSQL 17 with both `pgrdf` and `pgck` ready to use.

The launch contract for that image is:

- PostgreSQL starts successfully
- both extension files are present in the image
- both extensions are created automatically on first boot
- the default database is immediately usable
- the image runs on `linux/amd64` and `linux/arm64`

This repository should optimize toward that target from the first `core-pg17` decisions onward.

## Release Naming

Local names must start with `ociger-`.

Recommended image names:

- `ghcr.io/sporaxis-com/ociger-core-pg17-min`
- `ghcr.io/sporaxis-com/ociger-core-pg17-debug`
- `ghcr.io/sporaxis-com/ociger-bundle-pg17-pgrdf`
- `ghcr.io/sporaxis-com/ociger-triple-pg17-pgrdf-pgck`
- `ghcr.io/sporaxis-com/ociger-bundle-index`

Tags should include:

- semantic bundle version or repo release version
- explicit PostgreSQL major where relevant
- immutable digest references in documentation

## Matrix Structure

The repository documentation must keep a bundle matrix from the start.

Required columns:

- Bundle
- Kind
- Runnable
- Platforms
- PostgreSQL
- `pgrdf`
- `pgck`
- Init behavior
- Local smoke test
- Data storage proof
- Compressed size
- Uncompressed size
- Upstream inputs
- OCI reference
- Status

Seed rows:

| Bundle | Kind | Runnable | Platforms | PostgreSQL | pgrdf | pgck | Init behavior | Local smoke test | Data storage proof | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `core-pg17-min` | release image | yes | amd64, arm64 | 17 | no | no | initialize cluster on first boot | required | required | planned |
| `core-pg17-debug` | debug image | yes | amd64, arm64 | 17 | no | no | initialize cluster on first boot | required | optional | planned |
| `bundle-pg17-pgrdf` | release image | yes | amd64, arm64 | 17 | yes | no | extension auto-created on first boot | required | required | planned |
| `triple-pg17-pgrdf-pgck` | release image | yes | amd64, arm64 | 17 | yes | yes | both extensions auto-created on first boot | required | required | planned |
| `bundle-index` | OCI descriptor | no | not platform-specific | not embedded | references | references | no runtime initialization | not applicable | not applicable | planned |

## Documentation Requirements

The README must become the public control plane for the repository.

Minimum README sections after the first implementation:

- project overview
- bundle philosophy
- naming conventions
- local collision policy
- current bundle matrix
- exact launch commands
- `core-pg17` smoke test transcript
- data storage proof
- size table
- roadmap to the triple bundle
- publishing and provenance notes

## CI And Publishing Model

GitHub Actions should eventually provide:

- multi-arch Buildx builds
- public GHCR publishing
- immutable digests in release output
- release notes
- size measurements captured into docs or release assets

The first CI implementation only needs to cover the `core-pg17` bundle. Later workflows can extend the same generator-driven pattern to the extension bundles and final triple bundle.

## Risks And Constraints

1. PostgreSQL may require more runtime files than expected, limiting how close to true distroless the image can get.
2. Locale, NSS, timezone, and shared library requirements may impose a non-trivial size floor.
3. Upstream extension artifacts may need a materialization step outside direct Docker `FROM` usage because they are OCI artifacts rather than ordinary container images.
4. Size targets must be treated as measured outputs, not design-time promises.

These are acceptable constraints. The repository should document them rather than pretending they do not exist.

## Success Criteria

The first design milestone is successful when all of the following are true:

- the repository contains a clear bundle spec model
- the repository contains a generator-driven Docker build model
- `core-pg17-min` can be built for amd64 and arm64
- a local smoke test can create a database, table, and row
- the README shows where the row lives on disk
- the measured size appears in the public matrix

The broader program is successful when the triple bundle is public, runnable, and minimal enough to feel surprising.

## Roadmap

Phase 1:

- define the bundle spec
- generate `core-pg17` Dockerfile and Bake config
- build and verify `core-pg17-min`
- write the first measured matrix rows

Phase 2:

- add `core-pg17-debug`
- add generator support for OCI artifact materialization inputs
- define the `bundle-pg17-pgrdf` bundle

Phase 3:

- define the `triple-pg17-pgrdf-pgck` bundle
- publish the first public all-three runnable image
- add the future OCI bundle descriptor

This sequence keeps the current work small while preserving the main goal from the beginning.
