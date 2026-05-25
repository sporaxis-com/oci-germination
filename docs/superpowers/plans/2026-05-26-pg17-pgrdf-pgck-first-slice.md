# PG17 + pgRDF + pgCK First-Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and release a runnable `pg17-pgrdf-pgck` image that installs `pgRDF 0.5.1` and `pgCK 0.1.2`, preloads `pgck` by default, and proves both extension version surfaces locally and from the public image.

**Architecture:** Extend the existing generator-driven bundle system with a third bundle variant, add a launcher-supported preload flag for `pgck`, fetch `pgRDF` from the current pinned tarball path and `pgCK` from its published OCI artifact with ORAS, then add a local smoke path and a release workflow matching the existing core and `pgRDF` bundle flows.

**Tech Stack:** Go, Docker Buildx, distroless runtime, ORAS, GitHub Actions, bash smoke/build scripts.

---

### Task 1: Add the failing preload and triple-bundle render tests

**Files:**
- Modify: `internal/launcher/identity_test.go`
- Modify: `internal/bundle/render_test.go`
- Modify: `internal/bundle/spec.go`

- [ ] **Step 1: Write the failing launcher test**

Add a test that proves the launcher can append `shared_preload_libraries=pgck` without changing the existing default path:

```go
func TestPostgresArgsIncludeSharedPreloadLibrariesWhenRequested(t *testing.T) {
	args := PostgresArgs("/var/lib/postgresql/data", "pgck")

	want := []string{
		"/usr/lib/postgresql/17/bin/postgres",
		"-D", "/var/lib/postgresql/data",
		"-c", "shared_preload_libraries=pgck",
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: %#v", args)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/launcher
```

Expected: FAIL because `PostgresArgs` does not exist yet.

- [ ] **Step 3: Write the failing triple-bundle render test**

Add a render test proving the generated Dockerfile includes:

- a `pgck_fetch` stage using `ghcr.io/oras-project/oras:v1.2.2`
- an `oras pull ghcr.io/styk-tv/pgck:0.1.2-pg17-${TARGETARCH}` command
- copies for `pgck.so` and `pgck.control`
- `ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgck`

- [ ] **Step 4: Run the bundle tests to verify they fail**

Run:

```bash
go test ./internal/bundle
```

Expected: FAIL because the triple-bundle render path does not exist yet.

- [ ] **Step 5: Commit**

```bash
git add internal/launcher/identity_test.go internal/bundle/render_test.go internal/bundle/spec.go
git commit -m "test: add triple bundle render expectations"
```

### Task 2: Implement launcher preload support and triple-bundle generation

**Files:**
- Modify: `cmd/ociger-pg-launcher/main.go`
- Create: `internal/launcher/postgres_args.go`
- Modify: `internal/bundle/load.go`
- Modify: `internal/bundle/render.go`
- Modify: `scripts/generate.sh`
- Create: `bundles/bundle-pg17-pgrdf-pgck/bundle.yaml`
- Create: `bundles/bundle-pg17-pgrdf-pgck/.gitignore`

- [ ] **Step 1: Implement the launcher helper**

Create a focused helper:

```go
package launcher

func PostgresArgs(pgData string, preload string) []string {
	args := []string{"/usr/lib/postgresql/17/bin/postgres", "-D", pgData}
	if preload != "" {
		args = append(args, "-c", "shared_preload_libraries="+preload)
	}
	return args
}
```

- [ ] **Step 2: Use the helper in the launcher**

Replace the current direct `dropPrivilegesAndExec(..., bin("postgres"), "-D", pgData)` call with:

```go
preload := os.Getenv("OCIGER_SHARED_PRELOAD_LIBRARIES")
args := launcher.PostgresArgs(pgData, preload)
dropPrivilegesAndExec(runUID, runGID, args[0], args[1:]...)
```

- [ ] **Step 3: Extend bundle rendering**

Add a third bundle variant that:

- reuses the current `pgRDF` fetch stage
- adds:

```dockerfile
FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull "ghcr.io/styk-tv/pgck:0.1.2-pg17-${TARGETARCH}"
```

- copies:

```dockerfile
COPY --from=pgck_fetch /work/lib/pgck.so /out/usr/lib/postgresql/17/lib/pgck.so
COPY --from=pgck_fetch /work/share/extension/ /out/usr/share/postgresql/17/extension/
```

- sets:

```dockerfile
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgck
```

- [ ] **Step 4: Add the new bundle spec**

Create `bundles/bundle-pg17-pgrdf-pgck/bundle.yaml` using the existing layout and:

```yaml
name: bundle-pg17-pgrdf-pgck
description: PostgreSQL 17 with pgRDF and pgCK installed from upstream published artifacts
image:
  registry: ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: gcr.io/distroless/base-debian12:latest
platforms:
  - linux/amd64
  - linux/arm64
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-pg17-pgrdf-pgck-smoke/pgdata
  network: ociger-pg17-pgrdf-pgck-net
  container: ociger-pg17-pgrdf-pgck-smoke
```

- [ ] **Step 5: Regenerate and verify tests**

Run:

```bash
bash scripts/generate.sh
go test ./internal/launcher ./internal/bundle
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/ociger-pg-launcher/main.go internal/launcher/postgres_args.go internal/launcher/identity_test.go internal/bundle/load.go internal/bundle/render.go internal/bundle/render_test.go scripts/generate.sh bundles/bundle-pg17-pgrdf-pgck
git commit -m "feat: generate triple pg bundle"
```

### Task 3: Add local build and smoke verification

**Files:**
- Create: `scripts/build-pg17-pgrdf-pgck.sh`
- Create: `scripts/smoke-pg17-pgrdf-pgck.sh`

- [ ] **Step 1: Write the smoke script first**

The script must:

- create contained `ociger-` resources
- boot the image
- assert `pg_available_extensions` rows exist for both extensions
- run:

```sql
CREATE EXTENSION pgrdf;
CREATE EXTENSION pgck CASCADE;
```

- assert exact values:

```text
pgrdf default_version=0.5.1
pgck default_version=0.1.2
pgrdf extversion=0.5.1
pgck extversion=0.1.2
pgrdf.version()=0.5.1
pgck_version()=pgck 0.1.2 (rc3)
```

- [ ] **Step 2: Run it before implementation to verify failure**

Run:

```bash
bash scripts/smoke-pg17-pgrdf-pgck.sh
```

Expected: FAIL because the local image does not exist yet.

- [ ] **Step 3: Add the build script**

Mirror the current native build scripts and target:

```bash
docker buildx build \
  --load \
  --platform "$platform" \
  -f bundles/bundle-pg17-pgrdf-pgck/Dockerfile \
  -t ociger-pg17-pgrdf-pgck:local \
  .
```

- [ ] **Step 4: Build and run the smoke**

Run:

```bash
bash scripts/build-pg17-pgrdf-pgck.sh
bash scripts/smoke-pg17-pgrdf-pgck.sh
```

Expected: PASS with both extension version surfaces present.

- [ ] **Step 5: Commit**

```bash
git add scripts/build-pg17-pgrdf-pgck.sh scripts/smoke-pg17-pgrdf-pgck.sh
git commit -m "feat: add local triple bundle smoke"
```

### Task 4: Add release workflow, docs, and public verification

**Files:**
- Create: `.github/workflows/pg17-pgrdf-pgck-release.yml`
- Modify: `README.md`

- [ ] **Step 1: Add the new workflow**

Mirror the current release style with:

- verify on `pull_request`, `push`, `workflow_dispatch`, `schedule`
- publish on `pg17-pgrdf-pgck-v*`
- image `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck`
- `continue-on-error: true` on the GHCR publicization step

- [ ] **Step 2: Update the README**

Document:

- the new public image
- release tag `pg17-pgrdf-pgck-v0.1.1`
- platforms `amd64`, `arm64`
- the bundle chain now including the triple bundle
- the exact smoke output for `pgck`

- [ ] **Step 3: Run full verification**

Run:

```bash
go test ./...
bash scripts/generate.sh
bash scripts/build-pg17-pgrdf-pgck.sh
bash scripts/smoke-pg17-pgrdf-pgck.sh
```

Expected: PASS.

- [ ] **Step 4: Push and release**

Run:

```bash
git push origin main
git tag pg17-pgrdf-pgck-v0.1.1
git push origin pg17-pgrdf-pgck-v0.1.1
```

- [ ] **Step 5: Verify the public image**

Run:

```bash
docker logout ghcr.io >/dev/null 2>&1 || true
docker pull ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1
docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1
bash scripts/smoke-pg17-pgrdf-pgck.sh ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1
```

Expected: PASS with both extensions reporting the exact expected values.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/pg17-pgrdf-pgck-release.yml README.md
git commit -m "feat: release triple pg bundle"
```
