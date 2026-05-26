# SPEC.OCI.BUNDLE.v0.1 — Sporaxis-Com Compatible OCI Image Definition

**Version:** 0.1  
**Date:** 2026-05-26  
**Status:** Stable (as of pg17-pgrdf-pgck-nats and pg17-pgrdf-pgck-nats-micro public releases)

---

## 1. Overview

A **Sporaxis-Com compatible OCI image** is a containerized application bundle that follows the composition model of core runtime + optional extensions + optional services. Each bundle is defined by a `bundle.yaml` specification file and rendered into a multi-stage `Dockerfile` that produces a minimal, deterministic OCI image artifact.

The specification governs:
- How bundles are declared and composed
- How extensions (PostgreSQL plugins) are included
- How services (NATS, etc.) are colocated
- How images are built, tested, and published
- What lifecycle guarantees (ports, data directories, configuration) the image provides

### 1.1 Design Goals

- **Composability**: Extensions and services are declared independently; the build system combines them.
- **Minimalism**: Use distroless final images; include only what's necessary at runtime.
- **Multi-platform**: Every bundle MUST support `linux/amd64` and `linux/arm64` out of the box.
- **Testability**: Every bundle includes local smoke-test scripts and verification proofs.
- **Declarativeness**: The `bundle.yaml` is the source of truth; Dockerfiles are generated, not hand-edited.
- **Reproducibility**: Pinned upstream versions, deterministic fetches, signed/verified artifacts.

---

## 2. Bundle Anatomy

### 2.1 Directory Structure

Each bundle occupies a directory under `bundles/` with this shape:

```
bundles/<bundle-name>/
├── bundle.yaml                 ← declarative specification (this file is the source of truth)
├── Dockerfile                  ← generated from bundle.yaml (do not hand-edit)
└── smoke.sh                    ← optional: local verification script
```

The `<bundle-name>` follows the pattern `{core,bundle}-<descriptor>`, e.g.:
- `core-pg17` — PostgreSQL 17 core (minimal runtime)
- `bundle-pg17-pgrdf-pgck-nats` — PostgreSQL 17 + pgRDF + pgCK + NATS (all-in-one)
- `bundle-pg17-pgrdf-pgck-nats-micro` — PostgreSQL 17 + pgRDF + pgCK + NATS (size-optimized)

### 2.2 The `bundle.yaml` Schema

Every bundle is declared in YAML. The canonical schema is defined in `internal/bundle/spec.go` (Go struct tags) and rendered by the `bundle` CLI tool.

#### Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Unique bundle identifier (matches directory name) |
| `description` | string | yes | Human-readable summary of what the bundle includes |
| `image` | object | yes | OCI image metadata and registry details |
| `extensions` | object | no | PostgreSQL extensions to include (e.g., pgRDF, pgCK) |
| `services` | object | no | Colocated services (e.g., NATS) |
| `platforms` | list of strings | yes | Target architectures (e.g., `["linux/amd64", "linux/arm64"]`) |
| `ports` | list of objects | no | Port declarations for services and databases |
| `local` | object | yes | Local development / smoke-test configuration |

#### `image` Section

```yaml
image:
  registry: ghcr.io/sporaxis-com/ociger-<bundle-name>   # OCI registry + image name (no tag)
  pg_major: 17                                           # PostgreSQL major version
  base_image: postgres:17-bookworm                       # Upstream Docker image for build stage
  final_image: gcr.io/distroless/base-debian12:latest   # Distroless base for runtime
  runtime_profile: stable | micro                        # "stable" (full postgres) or "micro" (stripped)
```

**Semantics:**
- `registry`: Full registry path and image name, without the tag. The tag is determined at build/release time (e.g., `:v0.1.1`).
- `pg_major`: The major version of PostgreSQL (e.g., 16, 17). Used to resolve extension compatibility and binary paths.
- `base_image`: The upstream Docker image used in the initial build stage. Must be a tagged image on Docker Hub or a compatible registry.
- `final_image`: The distroless (or minimal) image used as the final base. Typically `gcr.io/distroless/base-debian12:latest`. Kept fixed to minimize attack surface and image size.
- `runtime_profile`: If `micro`, the build stage performs selective binary/library extraction (see §4.2). If `stable`, all PostgreSQL binaries and libraries are included.

#### `extensions` Section (Optional)

Extensions are PostgreSQL plugins. Currently supported:

```yaml
extensions:
  pgrdf:
    version: 0.5.1
  pgck:
    version: 0.1.2
```

**Semantics:**
- `pgrdf.version`: Version tag of the pgRDF extension. The build stage fetches the release from `https://github.com/styk-tv/pgRDF/releases/download/v<version>/pgrdf-<version>-pg<pg_major>-glibc-<arch>.tar.gz`.
- `pgck.version`: Version tag of the pgCK extension. The build stage pulls the OCI artifact from `ghcr.io/styk-tv/pgck:<version>-pg<pg_major>-<arch>` using `oras`.

Extensions are optional. A bundle MAY declare zero, one, or multiple extensions.

#### `services` Section (Optional)

Services are colocated processes (not PostgreSQL extensions, but separate long-running binaries).

```yaml
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
    jetstream: false
```

**Semantics:**
- `source_image`: The upstream OCI image to pull binaries from. The final Dockerfile uses `COPY --from=<stage>` to extract only the binary (e.g., `nats-server` from `nats:2.14.1-scratch`).
- `core_port`: The port on which the service listens for core protocol (e.g., NATS core protocol on 4222).
- `websocket_port`: The port on which the service listens for WebSocket/HTTP (e.g., NATS WebSocket on 9222).
- `jetstream`: Boolean; if `true`, the generated config enables NATS JetStream. Currently always `false`; reserved for future use.

Services are optional. A bundle MAY declare zero or one service (currently only NATS is supported).

#### `ports` Section (Optional)

Port declarations for service discovery and documentation. Format:

```yaml
ports:
  - name: postgres
    container_port: 5432
  - name: nats
    container_port: 4222
  - name: nats-ws
    container_port: 9222
```

**Semantics:**
- `name`: Human-readable service name (used in documentation).
- `container_port`: The port inside the container (matched to `services[].*.port` fields).

Ports are optional but recommended for documentation. The Dockerfile EXPOSE instruction is generated from this section.

#### `platforms` Section (Required)

Target architectures for multi-platform builds:

```yaml
platforms:
  - linux/amd64
  - linux/arm64
```

**Requirement:** Every bundle MUST support both `linux/amd64` and `linux/arm64`. Bundles targeting fewer than two platforms are rejected by the build system.

#### `local` Section (Required)

Configuration for local development and smoke tests:

```yaml
local:
  prefix: ociger-                                  # Docker container name prefix
  data_dir: .artifacts/ociger-<bundle>-smoke/pgdata
  network: ociger-<bundle>-net
  container: ociger-<bundle>-smoke
```

**Semantics:**
- `prefix`: Prefix for container names when running locally (e.g., `ociger-pg17-...`).
- `data_dir`: Path (relative to repo root) where PostgreSQL data is mounted during local smoke tests.
- `network`: Docker network name for local containers (allows multi-container communication).
- `container`: Container name for local smoke tests (e.g., `ociger-pg17-pgrdf-pgck-nats-smoke`).

These values are used by the `scripts/smoke-*.sh` shell scripts to run the image locally without manual container configuration.

---

## 3. Rendering: From Spec to Dockerfile

The Dockerfile is **generated** from `bundle.yaml` by the `bundle` CLI tool (or equivalent). The template system respects these directives:

### 3.1 Build Stages

The Dockerfile is structured in multiple stages:

1. **launcher_build** (always present): Builds the `ociger-pg-launcher` binary (Go).
2. **supervisor_build** (if `hasNATS(spec)`): Builds the `ociger-supervisor` binary (Go).
3. **pgrdf_fetch** (if `includesPGRDF(spec)`): Fetches pgRDF from GitHub releases.
4. **pgck_fetch** (if `includesPGCK(spec)`): Fetches pgCK from ORAS registry.
5. **nats_source** (if `hasNATS(spec)`): Extracts nats-server from upstream image.
6. **postgres_source** (always present): Extracts PostgreSQL binaries (full or micro).
7. **final** (always present): Constructs the runtime image from distroless base.

### 3.2 Template Functions

The Dockerfile template uses these helper functions (implemented in Go):

| Function | Returns | Meaning |
|---|---|---|
| `hasNATS(spec)` | bool | True if `spec.Services.NATS` is declared |
| `includesPGRDF(spec)` | bool | True if `spec.Extensions.PGRDF` is declared |
| `includesPGCK(spec)` | bool | True if `spec.Extensions.PGCK` is declared |
| `isMicro(spec)` | bool | True if `spec.Image.RuntimeProfile == "micro"` |

### 3.3 Generation Workflow

1. The `bundle` CLI reads `bundle.yaml` and parses it into the `Spec` struct.
2. Template functions are evaluated against the Spec.
3. The Dockerfile template is rendered, substituting variables and conditionally including stages.
4. The rendered Dockerfile is written to `Dockerfile` (overwriting any prior version).
5. The build system then runs `docker buildx build` with `--platform linux/amd64,linux/arm64`.

### 3.4 Build-Time Verification

- All fetches are verified:
  - pgRDF: `.tar.gz` is extracted; `.so` and `.sql` files must exist.
  - pgCK: ORAS pull succeeds; `.so` and `.sql` files must exist.
  - NATS: Binary exists at `/usr/sbin/nats-server`.
- All extensions are copied to the correct PostgreSQL directories:
  - `lib/*.so` → `/usr/lib/postgresql/<pg_major>/lib/`
  - `share/extension/*.control` → `/usr/share/postgresql/<pg_major>/extension/`
  - `share/extension/*.sql` → `/usr/share/postgresql/<pg_major>/extension/`

---

## 4. Runtime Behavior

### 4.1 Container Lifecycle

When the image is started with `docker run` (or equivalent):

1. The container starts with the `ociger-pg-launcher` binary as entrypoint (if NATS is not present) or `ociger-supervisor` (if NATS is present).
2. If supervisor is present, it starts PostgreSQL as child process 1 and NATS as child process 2, both supervised.
3. PostgreSQL initializes the `PGDATA` directory if not already initialized.
4. NATS starts its core listener (default `0.0.0.0:4222`) and WebSocket listener (default `0.0.0.0:9222`).
5. The container enters steady state, with both services running and logging to stderr/stdout.

### 4.2 Configuration & Environment

The container respects these environment variables:

| Var | Default | Meaning |
|---|---|---|
| `PGDATA` | `/var/lib/postgresql/data` | PostgreSQL data directory (must be a mounted volume for persistence) |
| `POSTGRES_INITDB_ARGS` | (empty) | Arguments passed to `initdb` during first startup |
| `POSTGRES_INITDB_WALDIR` | (unset) | WAL directory (if set, separate from PGDATA) |

NATS configuration is minimal and fixed:
- Core listener: `0.0.0.0:4222`
- WebSocket listener: `0.0.0.0:9222`
- No persistent storage (message bus, not a database).

### 4.3 Data Persistence

**PostgreSQL**: The container expects `PGDATA` to be a mounted volume. On first start, `initdb` initializes it. Subsequent starts use the existing cluster.

**NATS**: No persistent state. If the container stops, in-flight messages are lost (unless JetStream is enabled, which is not yet implemented).

### 4.4 Micro Runtime Profile

If `runtime_profile: micro`, the **postgres_source** build stage performs selective extraction:

```dockerfile
RUN set -eux; \
  mkdir -p /out/bin /out/etc /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin ...
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres /out/...
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb /out/...
  # (copy only essential .so, .bki, .txt, .sql files)
```

This reduces the final image size by ~40–50%, eliminating development tools, documentation, and unused shared libraries. Trade-off: some contrib modules and advanced features may not be available. Extensions (pgRDF, pgCK) are still included if declared.

---

## 5. Lifecycle: Build → Test → Release

### 5.1 Local Build

```bash
# Render Dockerfile from bundle.yaml (or regenerate if spec changed)
bundle render bundles/<bundle-name>

# Build locally (single-platform, for immediate testing)
docker build -t <registry>/<bundle-name>:<tag> bundles/<bundle-name>
```

Or use the provided shell script:
```bash
bash scripts/build-<bundle-name>.sh
```

### 5.2 Local Smoke Test

```bash
# Run the image locally and verify basic functionality
bash scripts/smoke-<bundle-name>.sh [<image-tag>]
```

If `<image-tag>` is omitted, the script uses the default local build tag. If provided, it pulls and tests the public image.

The smoke test verifies:
- PostgreSQL startup and connectivity.
- Extension availability (if included): `CREATE EXTENSION`, `SELECT version()`.
- NATS availability (if included): socket test on core and WebSocket ports.
- Relation files are present and accessible on the host (via mounted volume).

### 5.3 Multi-Platform Build & Push

```bash
# Requires buildx with multi-platform support
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --push \
  --tag <registry>/<bundle-name>:<tag> \
  bundles/<bundle-name>
```

Or use the GitHub Actions workflow:

```yaml
# .github/workflows/<bundle-name>-release.yml
on:
  push:
    tags:
      - '<bundle-name>-v*'
```

When a tag matching `<bundle-name>-v*` is pushed, the workflow:
1. Renders the Dockerfile.
2. Runs `docker buildx` with `--push` to multi-platform registries (GHCR, etc.).
3. Publishes digests and manifests.

### 5.4 Post-Release Verification

After images are pushed to GHCR:

```bash
# Pull and test the public image (simulating end-user experience)
docker rmi <image>  # optional: remove local version first
bash scripts/smoke-<bundle-name>.sh <registry>/<bundle-name>:v<tag>
```

This verifies that the published image works identically to the local build (i.e., no registry/build-system-specific corruption).

---

## 6. Versioning & Tagging

### 6.1 Git Tags

Release tags follow this pattern:

```
<bundle-name>-v<semver>
```

Examples:
- `pg17-pgrdf-pgck-nats-v0.1.1`
- `pg17-pgrdf-pgck-nats-micro-v0.1.1`
- `core-pg17-v0.1.0`

Each bundle has its own version series (they are independent). Tags are pushed to the main repository.

### 6.2 OCI Image Tags

The OCI image itself is tagged:

```
ghcr.io/sporaxis-com/ociger-<bundle-name>:v<semver>
```

Additionally, **manifests** (multi-platform descriptors) are published to the same tag, allowing `docker pull` to auto-select the correct architecture.

### 6.3 Digests

Each image and manifest publish produces a **digest** (SHA256 hash of the content). Digests are immutable and published in release notes and README.

Example:
```
ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:v0.1.1
  sha256:8a7e8c42b3557a1b7958006ad42bf53423bd75512a9c3db530dbe0c6ae4f84bf
```

---

## 7. Adding a New Bundle

### 7.1 Checklist

To define a new Sporaxis-Com compatible bundle:

1. **Choose a name** following the pattern `{core,bundle}-<descriptor>`. Examples: `core-pg18`, `bundle-pg17-pgvector-nats`.

2. **Create directory** under `bundles/`:
   ```bash
   mkdir -p bundles/<bundle-name>
   ```

3. **Write `bundle.yaml`**:
   - Fill in all required top-level fields (`name`, `description`, `image`, `platforms`, `local`).
   - Declare extensions if needed (`extensions.pgrdf.*`, `extensions.pgck.*`).
   - Declare services if needed (`services.nats.*`).
   - Ensure `image.pg_major` matches the PostgreSQL version in `image.base_image`.
   - Ensure `image.registry` is fully qualified (no tag).
   - Ensure `platforms` includes both `linux/amd64` and `linux/arm64`.

4. **Render the Dockerfile**:
   ```bash
   bundle render bundles/<bundle-name>
   ```
   This generates `bundles/<bundle-name>/Dockerfile` from the spec.

5. **Create smoke test** (optional but recommended):
   Write `bundles/<bundle-name>/smoke.sh` (shell script). Reference existing smoke tests (e.g., `scripts/smoke-pg17-pgrdf-pgck-nats.sh`) for structure.

6. **Write build script**:
   Create `scripts/build-<bundle-name>.sh` that invokes `docker build`. Reference existing build scripts for structure.

7. **Test locally**:
   ```bash
   bash scripts/build-<bundle-name>.sh
   bash scripts/smoke-<bundle-name>.sh
   ```

8. **Create release workflow** (if publishing to GHCR):
   Write `.github/workflows/<bundle-name>-release.yml`. Reference existing workflows (e.g., `pg17-pgrdf-pgck-nats-release.yml`).

9. **Tag and push**:
   ```bash
   git tag <bundle-name>-v<semver>
   git push origin <bundle-name>-v<semver>
   ```

10. **Verify public release**:
    Pull and test from GHCR:
    ```bash
    docker rmi ghcr.io/sporaxis-com/ociger-<bundle-name>:v<semver>  # optional
    bash scripts/smoke-<bundle-name>.sh ghcr.io/sporaxis-com/ociger-<bundle-name>:v<semver>
    ```

11. **Update README** with new bundle metadata, launch example, and verification command.

### 7.2 Validation Rules

The build system MUST enforce:

- [ ] `bundle.yaml` is valid YAML and conforms to the schema.
- [ ] `image.pg_major` is an integer ≥ 12.
- [ ] `image.base_image` points to a valid upstream PostgreSQL image.
- [ ] `image.final_image` is a distroless or minimal image.
- [ ] `image.registry` does not contain a tag (`:` at end is rejected).
- [ ] `platforms` is non-empty and contains only `linux/amd64` and/or `linux/arm64`.
- [ ] If extensions are declared, versions are non-empty strings.
- [ ] If NATS is declared, `core_port` and `websocket_port` are valid ports (1–65535).
- [ ] `local.prefix`, `local.data_dir`, `local.network`, `local.container` are all non-empty strings.
- [ ] The rendered Dockerfile is syntactically valid.

---

## 8. Extension Compatibility Matrix

| Extension | Min PG | Max PG | Fetch Method | Verified Versions |
|---|---|---|---|---|
| pgRDF | 13 | 17 | GitHub release (tar.gz) | 0.5.1 |
| pgCK | 13 | 17 | ORAS pull | 0.1.2 |

Future extensions:
- pgvector (APL 2.0 license, widely used)
- pgvectorscale (Timescale extension)
- pgtap (PostgreSQL testing framework)

---

## 9. Known Limitations & Future Work

### 9.1 Current Limitations

- **Single service per bundle**: `services.nats` is the only supported service. To add PostgreSQL replication, separate bundle definitions are needed.
- **No JetStream**: NATS is deployed in core mode only (`jetstream: false` is required).
- **No PostGIS**: Not yet included in extension matrix (requires GEOS, PROJ; adds significant size).
- **No custom init scripts**: Bundles do not yet support per-bundle `docker-entrypoint-initdb.d/` hooks.

### 9.2 Planned Extensions

- [ ] pgvector support (embedding vector type for AI/ML).
- [ ] JetStream config option for NATS.
- [ ] PostGIS with size-conscious delivery (micro mode).
- [ ] Custom init script hooks.
- [ ] Health check declarations in `bundle.yaml`.
- [ ] Multi-service support (e.g., Redis + PostgreSQL + NATS in one image).

---

## 10. Testing & Compliance

### 10.1 Automated Tests

The repository includes:
- **`internal/bundle/render_test.go`**: Unit tests for Dockerfile rendering (validates template logic, extension inclusion, platform matrix).
- **`internal/bundle/load_test.go`**: Unit tests for YAML parsing and schema validation.
- **Smoke test scripts** (`scripts/smoke-*.sh`): Integration tests that verify image startup and functionality.

Run tests:
```bash
go test -v ./internal/bundle
bash scripts/smoke-pg17-pgrdf-pgck-nats.sh
```

### 10.2 Image Attestation

Each published image includes:
- **Multi-platform manifest**: Allows `docker pull` to auto-detect and pull the correct architecture.
- **Digest**: SHA256 hash of the image content. Immutable; published in release notes.
- **Build info**: Buildkit metadata (source git commit, build timestamp, builder version).

---

## 11. FAQ

### Q: Can I include multiple extensions?

**A:** Yes. Declare them all in `extensions`:
```yaml
extensions:
  pgrdf:
    version: 0.5.1
  pgck:
    version: 0.1.2
```
The build stage will fetch and install both.

### Q: Can I use a non-distroless final image?

**A:** No. The spec requires `final_image` to be a minimal/distroless image (e.g., `gcr.io/distroless/base-debian12:latest`). This is a security and size requirement. If you need development tools, use a separate build artifact or extend in a downstream image.

### Q: How do I update an extension version?

**A:** Edit `bundle.yaml`, update the version string, and regenerate:
```bash
bundle render bundles/<bundle-name>
# Test locally
bash scripts/build-<bundle-name>.sh && bash scripts/smoke-<bundle-name>.sh
# Tag and push
git tag <bundle-name>-v<new-semver>
git push origin <bundle-name>-v<new-semver>
```

### Q: Can I pin a service (e.g., NATS) to a specific version long-term?

**A:** Yes. Edit `services.nats.source_image` to a pinned digest:
```yaml
services:
  nats:
    source_image: nats:2.14.1-scratch@sha256:<digest>
```
This prevents upstream image changes from affecting your build.

### Q: What's the smallest image I can build?

**A:** Use `runtime_profile: micro`. Example sizes:
- `pg17-pgrdf-pgck-nats-micro:v0.1.1` on linux/arm64: 41.3 MiB compressed, 104.5 MiB uncompressed.

---

## 12. References

- [Bundle Rendering Code](internal/bundle/render.go) — Template logic and build stage definitions.
- [Bundle Spec Struct](internal/bundle/spec.go) — Go struct tags defining the YAML schema.
- [Example Bundles](bundles/) — Live examples: `core-pg17/`, `bundle-pg17-pgrdf-pgck-nats/`, `bundle-pg17-pgrdf-pgck-nats-micro/`.
- [README.md](README.md) — User-facing documentation, launch examples, digests, and sizes.
- [Smoke Tests](scripts/smoke-*.sh) — Integration test examples.
- [GitHub Actions Workflows](.github/workflows/) — Release and publishing automation.

---

**Document End. This spec is normative for all Sporaxis-Com OCI bundle definitions.**
