# Core PG17 NATS Bundle Variants Design

Date: 2026-05-26
Status: Approved design
Scope: add minimal PostgreSQL + NATS bundle variants with a reusable service layer pattern, starting with `core-pg17-nats` and `core-pg17-nats-micro`

## Problem Statement

The repository now has:

- a stable `core-pg17` runtime line
- a specified `core-pg17-micro` size-focused runtime line
- released `pgRDF` and `pgCK` extension bundle lines

The next requirement is to add NATS as an additional service that can run in the same container as PostgreSQL while preserving the repo's OCI-bundle approach.

The user wants:

- NATS included as a reusable add-on service pattern that can be applied to multiple bundles later
- an exposed WebSocket port on a second listener
- JetStream disabled for the first slice, but represented as a future matrix option
- aggressive size minimization
- no `systemd`

This means the repository needs a minimal multi-service runtime pattern that can later host one or two additional services the same way.

## Goals

- Add two new runnable bundle variants:
  - `core-pg17-nats`
  - `core-pg17-nats-micro`
- Keep the current `core-pg17` and `core-pg17-micro` lines available as separate non-NATS variants.
- Add a reusable NATS service layer pattern that can be attached to future bundles.
- Start PostgreSQL and NATS in the same container with one tiny static supervisor.
- Expose:
  - PostgreSQL on `5432`
  - NATS core on `4222`
  - NATS WebSocket listener on a second port
- Keep JetStream disabled by default in the first slice.
- Keep the final images as small as possible by copying only:
  - the PostgreSQL runtime files required for the selected base variant
  - the `nats-server` binary
  - a generated minimal NATS config
  - the tiny static launcher/supervisor
- Preserve the existing bind-mounted PostgreSQL data proof and local containment rules.
- Document the new bundle variants and future matrix rows for JetStream-capable releases.

## Non-Goals

- Using `systemd` or a full init suite.
- Shipping the NATS CLI in the final runtime image.
- Enabling JetStream in the first release.
- Solving TLS-backed `wss://` in the first slice.
- Adding `pgRDF` or `pgCK` to the NATS bundles in this slice.
- Inventing a low-level OCI assembly engine outside normal Dockerfile and BuildKit mechanics.

## Design Summary

The repository should treat NATS as a reusable service capability rather than a one-off image fork.

That service capability will first materialize as two concrete bundle outputs:

- `core-pg17-nats`
- `core-pg17-nats-micro`

The service capability should be designed so later bundles can adopt the same NATS runtime pattern without inventing a different supervision approach.

The first NATS release line is:

- PostgreSQL + NATS core + NATS WebSocket
- no JetStream
- no TLS
- local/dev defaults only

## Architecture

### Bundle Variants

The first concrete variants are:

```text
bundles/core-pg17-nats/
bundles/core-pg17-nats-micro/
```

They are derived conceptually from:

- `core-pg17`
- `core-pg17-micro`

plus a shared NATS service layer contract.

The generator should continue to emit normal OCI build outputs:

- `bundle.yaml`
- `Dockerfile`
- `docker-bake.hcl`

### Service Runtime Model

BuildKit remains a build-time assembler only.

At runtime, the container needs one static supervisor process that:

1. starts PostgreSQL
2. starts `nats-server`
3. forwards signals
4. exits non-zero if either child process exits unexpectedly
5. terminates the sibling process during shutdown or failure

The design explicitly rejects:

- `systemd`
- shell-heavy wrapper scripts in the final image
- multiple unrelated supervision mechanisms per bundle

The same static supervisor should be reusable for later service-enabled bundles.

## NATS Service Layer

### Input Strategy

The NATS service layer should consume the upstream NATS OCI image only as a source stage.

It should copy only:

- the `nats-server` binary

It should not copy:

- the full upstream NATS image filesystem
- example files
- package-manager artifacts
- CLI helpers

### Generated Config

The service layer should generate a minimal NATS config file in the final image.

First-slice config contract:

- listen on `0.0.0.0:4222` for core NATS
- enable a WebSocket listener on `0.0.0.0:9222`
- disable JetStream
- no monitoring port by default
- no clustering, gateways, or leafnodes
- local/dev auth defaults only for smoke use

The WebSocket listener must be exposed as a second port in the bundle metadata and Dockerfile.

### WebSocket and `wss`

The first slice must expose a WebSocket-capable second port.

The first release uses plain WebSocket transport suitable for local/dev smoke use.

Future matrix rows should reserve:

- `+wss`
- `+jetstream`
- `+wss+jetstream`

Those future rows do not need to be implemented in this slice.

## Bundle Contracts

### `core-pg17-nats`

This variant builds from the current stable `core-pg17` runtime shape and adds:

- `nats-server`
- generated minimal NATS config
- service supervisor

Its purpose is:

- a stable PostgreSQL + NATS line
- faster delivery
- baseline size comparison against the micro line

### `core-pg17-nats-micro`

This variant builds from the `core-pg17-micro` runtime shape and adds the same NATS service layer.

Its purpose is:

- the most size-focused PostgreSQL + NATS line in this family
- the future base for additional minimal multi-service bundles

## Size Rules

Size reduction is a first-class acceptance criterion.

The implementation must minimize size by design:

- copy only `nats-server`, not the full NATS image
- no `systemd`
- no NATS CLI in the runtime image
- no shell in the final runtime image unless verification proves it is required
- no broad library-directory copies in the micro line
- no duplicated base layers that can be avoided by using `scratch`-style final stages where applicable

The README and matrix must record measured sizes after build rather than estimated sizes.

The first NATS release line should prefer correctness with aggressive pruning, not feature breadth.

## Ports and Runtime Defaults

Required ports:

- `5432` PostgreSQL
- `4222` NATS core
- `9222` NATS WebSocket

Not enabled by default in the first slice:

- JetStream storage
- monitoring port
- TLS for `wss://`

These defaults keep both runtime size and operational surface smaller.

## Data and Persistence

PostgreSQL continues to use:

- `PGDATA=/var/lib/postgresql/data`

NATS in the first slice is ephemeral.

No NATS data directory is required while JetStream stays disabled.

Future JetStream-capable rows can introduce a dedicated bind-mounted NATS data path and the corresponding matrix documentation.

## Verification Contract

### `core-pg17-nats`

The first stable NATS smoke contract must prove:

1. PostgreSQL boots
2. NATS accepts a TCP client connection on `4222`
3. the WebSocket listener is bound on `9222`
4. PostgreSQL still passes the existing SQL and relation-file persistence proof

### `core-pg17-nats-micro`

The first micro NATS smoke contract must prove the same points:

1. PostgreSQL boots
2. NATS accepts a TCP client connection on `4222`
3. the WebSocket listener is bound on `9222`
4. PostgreSQL still passes the SQL and relation-file persistence proof

The first NATS slice does not need a full WebSocket messaging smoke if the port is actively listening and the core NATS path is proven.

That narrower contract is acceptable because the immediate requirement is to expose the second port and establish the reusable service pattern without over-expanding scope.

## Local Resource Rules

New smoke resources must stay contained to `ociger-` names and `.artifacts/ociger-*` paths.

Recommended local names:

### `core-pg17-nats`

- image: `ociger-core-pg17-nats:local`
- container: `ociger-core-pg17-nats-smoke`
- network: `ociger-core-pg17-nats-net`
- data dir: `.artifacts/ociger-core-pg17-nats-smoke/pgdata`

### `core-pg17-nats-micro`

- image: `ociger-core-pg17-nats-micro:local`
- container: `ociger-core-pg17-nats-micro-smoke`
- network: `ociger-core-pg17-nats-micro-net`
- data dir: `.artifacts/ociger-core-pg17-nats-micro-smoke/pgdata`

## CI and Release

The repository should add two workflows:

- `.github/workflows/core-pg17-nats-release.yml`
- `.github/workflows/core-pg17-nats-micro-release.yml`

Both should follow the existing release style:

- verify on `pull_request`, `push`, `workflow_dispatch`, and `schedule`
- publish on tags matching:
  - `core-pg17-nats-v*`
  - `core-pg17-nats-micro-v*`

Planned image names:

- `ghcr.io/sporaxis-com/ociger-core-pg17-nats`
- `ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro`

Planned first release tags:

- `core-pg17-nats-v0.1.0`
- `core-pg17-nats-micro-v0.1.0`

Each workflow must:

1. run `go test ./...`
2. regenerate bundle outputs and verify they are committed
3. build the native-arch image
4. run the bundle-specific smoke test
5. publish multi-arch images for `linux/amd64,linux/arm64`
6. tag the image as:
   - `v0.1.0`
   - `latest`
7. follow the existing GHCR visibility/publicization tolerance rules after successful push

## Feature Matrix Additions

The README matrix should add rows for:

- `core-pg17-nats`
- `core-pg17-nats-micro`
- future `core-pg17-nats+jetstream`
- future `core-pg17-nats-micro+jetstream`
- future `core-pg17-nats+wss`
- future `core-pg17-nats-micro+wss`

The first release marks only the first two as released if they pass verification.

Future rows remain planned.

## Risks and Tradeoffs

### Multi-Service Complexity

Running PostgreSQL and NATS in one container is more complex than the one-process bundles.

That complexity is accepted because the user explicitly wants a reusable service-start pattern for multiple colocated services.

### WebSocket Scope

Full WebSocket messaging verification would expand the first slice.

The first slice therefore verifies the second listener is exposed and live while keeping deeper WSS validation for a later targeted row.

### Size Pressure Versus Runtime Safety

Aggressive pruning can accidentally remove files that are not obvious at first glance.

That is why the first NATS bundles must prove both:

- PostgreSQL persistence behavior still works
- NATS binds and accepts client connections

### JetStream Creep

JetStream is intentionally held out of the first release because it changes persistence, config, and matrix complexity.

The matrix should acknowledge it without activating it.

## Acceptance Criteria

This slice is complete when all of the following are true:

1. the repo contains committed generated outputs for:
   - `bundles/core-pg17-nats/`
   - `bundles/core-pg17-nats-micro/`
2. local `go test ./...` passes
3. local native-arch builds succeed for both variants
4. both local smoke tests pass
5. measured sizes are recorded for both variants
6. public multi-arch images are published for both variants
7. anonymous pulls of both public images succeed
8. smoke tests pass against both public images
9. the README documents both variants and the future JetStream/WSS matrix rows accurately
