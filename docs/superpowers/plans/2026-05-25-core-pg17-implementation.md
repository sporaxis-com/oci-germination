# Core PG17 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and publish a public `core-pg17` OCI image that is small, runnable on `linux/amd64` and `linux/arm64`, safe to test locally under the `ociger-` prefix, and proven by a fresh pull-and-run verification from GHCR.

**Architecture:** A small Go-based bundle generator renders the `core-pg17` Dockerfile and Bake config from `bundle.yaml`. The final image is assembled with a normal multi-stage Dockerfile that extracts only required PostgreSQL runtime assets from `postgres:17-bookworm`, compiles a tiny Go launcher for first-boot initialization, and runs a bind-mounted smoke test that proves PostgreSQL storage on disk. A tag-triggered GitHub Actions workflow publishes the multi-arch image to GHCR, after which the image is pulled fresh and verified locally.

**Tech Stack:** Go 1.24, Docker Buildx, PostgreSQL 17 official image, distroless Debian 12 base, GitHub Actions, GHCR

---

## File Structure

**Create:**
- `go.mod`
- `cmd/ociger-gen/main.go`
- `internal/bundle/spec.go`
- `internal/bundle/load.go`
- `internal/bundle/render.go`
- `internal/bundle/render_test.go`
- `cmd/ociger-pg-launcher/main.go`
- `bundles/core-pg17/bundle.yaml`
- `bundles/core-pg17/.gitignore`
- `scripts/generate.sh`
- `scripts/build-core-pg17.sh`
- `scripts/smoke-core-pg17.sh`
- `.github/workflows/core-pg17-release.yml`

**Create or overwrite generated files:**
- `bundles/core-pg17/Dockerfile`
- `bundles/core-pg17/docker-bake.hcl`

**Modify:**
- `README.md`
- `.gitignore`

**Test / verify:**
- `go test ./...`
- `bash scripts/generate.sh`
- `bash scripts/build-core-pg17.sh`
- `bash scripts/smoke-core-pg17.sh`
- `docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0`
- fresh `docker pull` and `docker run` using the public GHCR tag

### Task 1: Scaffold the generator and lock the bundle contract

**Files:**
- Create: `go.mod`
- Create: `cmd/ociger-gen/main.go`
- Create: `internal/bundle/spec.go`
- Create: `internal/bundle/load.go`
- Create: `internal/bundle/render.go`
- Test: `internal/bundle/render_test.go`

- [ ] **Step 1: Write the failing generator test**

```go
package bundle

import (
	"strings"
	"testing"
)

func TestRenderCoreBundle(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17",
		Description: "Minimal embedded PostgreSQL 17 runtime",
		Image: ImageSpec{
			Registry:   "ghcr.io/sporaxis-com/ociger-core-pg17-min",
			PGMajor:    17,
			BaseImage:  "postgres:17-bookworm",
			FinalImage: "gcr.io/distroless/base-debian12:latest",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Local: LocalSpec{
			Prefix:    "ociger-",
			DataDir:   ".artifacts/ociger-core-pg17-smoke/pgdata",
			Network:   "ociger-core-pg17-net",
			Container: "ociger-core-pg17-smoke",
		},
	}

	df, bake, err := Render(spec)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	if !strings.Contains(df, "FROM postgres:17-bookworm AS postgres_source") {
		t.Fatalf("Dockerfile missing postgres source stage:\n%s", df)
	}
	if !strings.Contains(df, "COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher") {
		t.Fatalf("Dockerfile missing launcher copy:\n%s", df)
	}
	if !strings.Contains(bake, "linux/amd64") || !strings.Contains(bake, "linux/arm64") {
		t.Fatalf("Bake output missing platforms:\n%s", bake)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./...
```

Expected: FAIL because `Spec`, `ImageSpec`, `LocalSpec`, and `Render` do not exist yet.

- [ ] **Step 3: Create the module file**

```go
module github.com/sporaxis-com/oci-germination

go 1.24.4

require gopkg.in/yaml.v3 v3.0.1
```

- [ ] **Step 4: Implement the spec types**

```go
package bundle

type Spec struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Image       ImageSpec `yaml:"image"`
	Platforms   []string  `yaml:"platforms"`
	Local       LocalSpec `yaml:"local"`
}

type ImageSpec struct {
	Registry   string `yaml:"registry"`
	PGMajor    int    `yaml:"pg_major"`
	BaseImage  string `yaml:"base_image"`
	FinalImage string `yaml:"final_image"`
}

type LocalSpec struct {
	Prefix    string `yaml:"prefix"`
	DataDir   string `yaml:"data_dir"`
	Network   string `yaml:"network"`
	Container string `yaml:"container"`
}
```

- [ ] **Step 5: Implement spec loading**

```go
package bundle

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Spec, error) {
	var spec Spec

	data, err := os.ReadFile(path)
	if err != nil {
		return spec, err
	}

	err = yaml.Unmarshal(data, &spec)
	return spec, err
}
```

- [ ] **Step 6: Implement rendering**

```go
package bundle

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
)

const dockerfileTemplate = `# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS launcher_build
WORKDIR /src
COPY go.mod ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

FROM {{ .Image.BaseImage }} AS postgres_source

FROM {{ .Image.FinalImage }}
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]
`

const bakeTemplate = `group "default" {
  targets = ["core-pg17"]
}

target "core-pg17" {
  context = "../.."
  dockerfile = "bundles/core-pg17/Dockerfile"
  tags = ["{{ .Image.Registry }}:dev"]
  platforms = [{{ range $i, $p := .Platforms }}{{ if $i }}, {{ end }}"{{ $p }}"{{ end }}]
}
`

func Render(spec Spec) (string, string, error) {
	df, err := executeTemplate(dockerfileTemplate, spec)
	if err != nil {
		return "", "", err
	}
	bake, err := executeTemplate(bakeTemplate, spec)
	if err != nil {
		return "", "", err
	}
	return df, bake, nil
}

func executeTemplate(source string, spec Spec) (string, error) {
	tpl, err := template.New("tpl").Parse(source)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, spec); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

func Write(spec Spec, dockerfilePath string, bakePath string) error {
	df, bake, err := Render(spec)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dockerfilePath, []byte(df), 0o644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}
	if err := os.WriteFile(bakePath, []byte(bake), 0o644); err != nil {
		return fmt.Errorf("write bake: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Implement the generator CLI**

```go
package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/sporaxis-com/oci-germination/internal/bundle"
)

func main() {
	specPath := flag.String("bundle", "", "Path to bundle.yaml")
	flag.Parse()

	if *specPath == "" {
		log.Fatal("--bundle is required")
	}

	spec, err := bundle.Load(*specPath)
	if err != nil {
		log.Fatal(err)
	}

	dir := filepath.Dir(*specPath)
	if err := bundle.Write(spec, filepath.Join(dir, "Dockerfile"), filepath.Join(dir, "docker-bake.hcl")); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 8: Run the test to verify it passes**

Run:

```bash
go test ./...
```

Expected: PASS with `ok` output for the bundle package.

- [ ] **Step 9: Commit**

```bash
git add go.mod cmd/ociger-gen/main.go internal/bundle/spec.go internal/bundle/load.go internal/bundle/render.go internal/bundle/render_test.go
git commit -m "feat: add core bundle generator"
```

### Task 2: Add the core bundle spec, runtime launcher, and generated build files

**Files:**
- Create: `cmd/ociger-pg-launcher/main.go`
- Create: `bundles/core-pg17/bundle.yaml`
- Create: `bundles/core-pg17/.gitignore`
- Modify or create via generator: `bundles/core-pg17/Dockerfile`
- Modify or create via generator: `bundles/core-pg17/docker-bake.hcl`
- Create: `scripts/generate.sh`

- [ ] **Step 1: Write the failing generation command**

Run:

```bash
bash scripts/generate.sh
```

Expected: FAIL because `scripts/generate.sh`, `bundle.yaml`, and the launcher source do not exist yet.

- [ ] **Step 2: Add the core bundle spec**

```yaml
name: core-pg17
description: Minimal embedded PostgreSQL 17 runtime
image:
  registry: ghcr.io/sporaxis-com/ociger-core-pg17-min
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: gcr.io/distroless/base-debian12:latest
platforms:
  - linux/amd64
  - linux/arm64
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-core-pg17-smoke/pgdata
  network: ociger-core-pg17-net
  container: ociger-core-pg17-smoke
```

- [ ] **Step 3: Add the bundle-local ignore file**

```gitignore
tmp/
dist/
```

- [ ] **Step 4: Add the generator wrapper**

```bash
#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/ociger-gen --bundle bundles/core-pg17/bundle.yaml
```

- [ ] **Step 5: Add the tiny PostgreSQL launcher**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	postgresUID = 999
	postgresGID = 999
)

func main() {
	pgData := getenv("PGDATA", "/var/lib/postgresql/data")
	if err := os.MkdirAll(pgData, 0o700); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll("/var/run/postgresql", 0o775); err != nil {
		log.Fatal(err)
	}
	if err := os.Chown(pgData, postgresUID, postgresGID); err != nil {
		log.Fatal(err)
	}
	if err := os.Chown("/var/run/postgresql", postgresUID, postgresGID); err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(pgData, "PG_VERSION")); os.IsNotExist(err) {
		runAsPostgres("/usr/lib/postgresql/17/bin/initdb", "-D", pgData, "--username=postgres", "--auth=trust", "--locale=C", "--encoding=UTF8")
		appendConfig(filepath.Join(pgData, "postgresql.conf"), "listen_addresses='*'\nunix_socket_directories='/var/run/postgresql'\n")
		appendConfig(filepath.Join(pgData, "pg_hba.conf"), "host all all all trust\n")
	}

	runForeground(pgData)
}

func runForeground(pgData string) {
	cmd := exec.Command("/usr/lib/postgresql/17/bin/postgres", "-D", pgData)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: postgresUID, Gid: postgresGID},
	}
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func runAsPostgres(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: postgresUID, Gid: postgresGID},
	}
	if err := cmd.Run(); err != nil {
		log.Fatalf("%s failed: %v", name, err)
	}
}

func appendConfig(path string, extra string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if _, err := fmt.Fprint(f, extra); err != nil {
		log.Fatal(err)
	}
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
```

- [ ] **Step 6: Replace the generated Dockerfile template with the runtime-aware version in `internal/bundle/render.go`**

```go
const dockerfileTemplate = `# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS launcher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

FROM {{ .Image.BaseImage }} AS postgres_source
RUN set -eux; \
  mkdir -p /out/usr/lib/postgresql /out/usr/share/postgresql /out/etc /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -a /usr/lib/postgresql/{{ .Image.PGMajor }} /out/usr/lib/postgresql/; \
  cp -a /usr/share/postgresql/{{ .Image.PGMajor }} /out/usr/share/postgresql/; \
  cp /etc/passwd /out/etc/passwd; \
  cp /etc/group /out/etc/group; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres | awk '/=> \\/|^\\// {print $(NF-1)}' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb | awk '/=> \\/|^\\// {print $(NF-1)}' | sort -u | xargs -r -I '{}' cp --parents '{}' /out

FROM {{ .Image.FinalImage }}
ENV PGDATA=/var/lib/postgresql/data
COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]
`
```

- [ ] **Step 7: Generate the Dockerfile and Bake files**

Run:

```bash
bash scripts/generate.sh
```

Expected: `bundles/core-pg17/Dockerfile` and `bundles/core-pg17/docker-bake.hcl` are created without error.

- [ ] **Step 8: Commit**

```bash
git add cmd/ociger-pg-launcher/main.go bundles/core-pg17/bundle.yaml bundles/core-pg17/.gitignore bundles/core-pg17/Dockerfile bundles/core-pg17/docker-bake.hcl scripts/generate.sh internal/bundle/render.go
git commit -m "feat: add core pg17 runtime bundle"
```

### Task 3: Add the local build and smoke-test path

**Files:**
- Create: `scripts/build-core-pg17.sh`
- Create: `scripts/smoke-core-pg17.sh`

- [ ] **Step 1: Write the failing smoke test**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="$ROOT/.artifacts/ociger-core-pg17-smoke/pgdata"
NETWORK="ociger-core-pg17-net"
CONTAINER="ociger-core-pg17-smoke"
IMAGE="${1:-ociger-core-pg17-min:local}"

docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker network rm "$NETWORK" >/dev/null 2>&1 || true
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

docker network create "$NETWORK" >/dev/null
docker run -d --name "$CONTAINER" --network "$NETWORK" -e PGDATA=/var/lib/postgresql/data -v "$DATA_DIR:/var/lib/postgresql/data" "$IMAGE"
```

Run:

```bash
bash scripts/smoke-core-pg17.sh
```

Expected: FAIL because the local image does not exist yet.

- [ ] **Step 2: Add the local build wrapper**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

bash scripts/generate.sh
docker buildx build \
  --load \
  --platform linux/arm64 \
  -f bundles/core-pg17/Dockerfile \
  -t ociger-core-pg17-min:local \
  .
```

- [ ] **Step 3: Replace the smoke test with the full proof**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="$ROOT/.artifacts/ociger-core-pg17-smoke/pgdata"
NETWORK="ociger-core-pg17-net"
CONTAINER="ociger-core-pg17-smoke"
CLIENT="ociger-core-pg17-client"
IMAGE="${1:-ociger-core-pg17-min:local}"

cleanup() {
  docker rm -f "$CLIENT" >/dev/null 2>&1 || true
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}

cleanup
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"
docker network create "$NETWORK" >/dev/null

docker run -d \
  --name "$CONTAINER" \
  --network "$NETWORK" \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$DATA_DIR:/var/lib/postgresql/data" \
  "$IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if docker run --rm --name "$CLIENT" --network "$NETWORK" postgres:17-bookworm \
    pg_isready -h "$CONTAINER" -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker run --rm --name "$CLIENT" --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
CREATE DATABASE ociger_demo;
SQL

docker run --rm --name "$CLIENT" --network "$NETWORK" postgres:17-bookworm \
  psql -h "$CONTAINER" -U postgres -d ociger_demo -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE public.demo_rows (
  id integer primary key,
  note text not null
);
INSERT INTO public.demo_rows (id, note) VALUES (1, 'ociger smoke row');
TABLE public.demo_rows;
SELECT pg_relation_filepath('public.demo_rows'::regclass);
SQL
```

- [ ] **Step 4: Run the build to verify it creates the image**

Run:

```bash
bash scripts/build-core-pg17.sh
```

Expected: PASS and `ociger-core-pg17-min:local` appears in `docker images`.

- [ ] **Step 5: Run the smoke test to verify the runtime**

Run:

```bash
bash scripts/smoke-core-pg17.sh
```

Expected:
- PostgreSQL starts
- `ociger_demo` is created
- one row is inserted and printed
- `pg_relation_filepath('public.demo_rows'::regclass)` returns a relative path

- [ ] **Step 6: Measure the local image size**

Run:

```bash
docker image inspect ociger-core-pg17-min:local --format '{{.Size}}'
docker history --human ociger-core-pg17-min:local
```

Expected: numeric size output and a readable layer breakdown. Save the actual values for the README.

- [ ] **Step 7: Commit**

```bash
git add scripts/build-core-pg17.sh scripts/smoke-core-pg17.sh
git commit -m "feat: add local core pg17 smoke test"
```

### Task 4: Add public publishing from the repo and document the measured result

**Files:**
- Create: `.github/workflows/core-pg17-release.yml`
- Modify: `README.md`
- Modify: `.gitignore`

- [ ] **Step 1: Add local artifact ignores**

```gitignore
.worktrees/
docs/superpowers/plans/
.artifacts/
```

- [ ] **Step 2: Add the release workflow**

```yaml
name: core-pg17-release

on:
  push:
    tags:
      - "core-pg17-v*"

permissions:
  contents: write
  packages: write

jobs:
  release:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Generate Dockerfile
        run: bash scripts/generate.sh

      - name: Build and push multi-arch image
        uses: docker/build-push-action@v6
        with:
          context: .
          file: bundles/core-pg17/Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/sporaxis-com/ociger-core-pg17-min:${{ github.ref_name }}
            ghcr.io/sporaxis-com/ociger-core-pg17-min:latest

      - name: Inspect pushed image
        run: docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-core-pg17-min:${GITHUB_REF_NAME}
```

- [ ] **Step 3: Replace the bootstrap README with the first real public README**

```markdown
# OCI Germination

`oci-germination` publishes small OCI-delivered PostgreSQL bundles.

## Current release target

`core-pg17-min` is the first layer:

- embedded PostgreSQL 17
- public GHCR image
- `linux/amd64` and `linux/arm64`
- local smoke test under `ociger-` names only
- no host-port collision required

## Naming and local safety

All local test resources are prefixed with `ociger-`.

- network: `ociger-core-pg17-net`
- server container: `ociger-core-pg17-smoke`
- data dir: `.artifacts/ociger-core-pg17-smoke/pgdata`

## Bundle matrix

| Bundle | Kind | Runnable | Platforms | PostgreSQL | pgrdf | pgck | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `core-pg17-min` | release image | yes | amd64, arm64 | 17 | no | no | released |
| `core-pg17-debug` | debug image | yes | amd64, arm64 | 17 | no | no | planned |
| `bundle-pg17-pgrdf` | release image | yes | amd64, arm64 | 17 | yes | no | planned |
| `triple-pg17-pgrdf-pgck` | release image | yes | amd64, arm64 | 17 | yes | yes | planned |

## Local build

```bash
bash scripts/build-core-pg17.sh
```

## Local smoke test

```bash
bash scripts/smoke-core-pg17.sh
```

## Public run

```bash
docker run --rm --name ociger-core-pg17-smoke ghcr.io/sporaxis-com/ociger-core-pg17-min:latest
```
```

- [ ] **Step 4: Run local verification again after the docs and workflow changes**

Run:

```bash
go test ./...
bash scripts/generate.sh
bash scripts/build-core-pg17.sh
bash scripts/smoke-core-pg17.sh
```

Expected: all commands pass.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/core-pg17-release.yml README.md .gitignore
git commit -m "feat: add core pg17 release workflow"
```

### Task 5: Push, publish, and verify from the public GHCR image

**Files:**
- No new files required unless docs need measured-value updates after release

- [ ] **Step 1: Push the branch commits**

Run:

```bash
git push origin main
```

Expected: `main` updates on GitHub successfully.

- [ ] **Step 2: Create and push the release tag**

Run:

```bash
git tag core-pg17-v0.1.0
git push origin core-pg17-v0.1.0
```

Expected: the tag exists on the public repository and triggers `core-pg17-release`.

- [ ] **Step 3: Wait for the workflow to finish and verify the published image**

Run:

```bash
gh run list --workflow core-pg17-release.yml --limit 5
gh run watch --exit-status
docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0
```

Expected:
- workflow completes successfully
- image manifest exists publicly
- manifest lists both `linux/amd64` and `linux/arm64`

- [ ] **Step 4: Pull the public image fresh**

Run:

```bash
docker rmi ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0 || true
docker pull ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0
```

Expected: image pulls successfully from GHCR.

- [ ] **Step 5: Re-run the smoke test against the public tag**

Run:

```bash
bash scripts/smoke-core-pg17.sh ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0
```

Expected:
- the pulled public image starts
- database, table, and row creation succeed
- `pg_relation_filepath(...)` returns a path
- that relative path maps to `.artifacts/ociger-core-pg17-smoke/pgdata/<relative-path>`

- [ ] **Step 6: Capture the measured public image size**

Run:

```bash
docker image inspect ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0 --format '{{.Size}}'
docker history --human ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0
```

Expected: measured size output suitable for the README and final report.

- [ ] **Step 7: Update docs with the measured results if they changed**

```bash
git add README.md
git commit -m "docs: record measured core pg17 release size"
git push origin main
```

- [ ] **Step 8: Final verification**

Run:

```bash
git status --short --branch
git log --oneline --decorate -5
docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.0
```

Expected:
- worktree clean
- recent commits visible on `main`
- public image manifest still resolves
