# Core PG17 Micro and NATS Bundles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build, verify, and release three new OCI bundle variants: `core-pg17-micro`, `core-pg17-nats`, and `core-pg17-nats-micro`.

**Architecture:** Extend the existing generator-driven bundle system with a runtime profile (`stable` vs `micro`), optional NATS service metadata, and exposed port metadata. Keep PostgreSQL bootstrap in the existing `ociger-pg-launcher`, add a tiny static `ociger-supervisor` only for NATS-enabled bundles, render a generated `nats-server.conf`, and verify everything locally before publishing multi-arch public images and documenting measured sizes.

**Tech Stack:** Go, Docker Buildx, scratch and distroless runtime images, official `nats:2.14.1-scratch` source image, bash smoke/build scripts, GitHub Actions, GHCR.

---

## File Map

- `internal/bundle/spec.go`
  Adds bundle schema for runtime profile, exposed ports, and optional NATS service config.
- `internal/bundle/load.go`
  Loads the expanded YAML schema.
- `internal/bundle/load_test.go`
  Verifies YAML parsing for runtime profile, ports, and NATS service config.
- `internal/bundle/render.go`
  Renders Dockerfiles, Bake files, and optional generated NATS config files.
- `internal/bundle/render_test.go`
  Verifies rendered Dockerfiles for `core-pg17-micro`, `core-pg17-nats`, and `core-pg17-nats-micro`.
- `internal/nats/config.go`
  Renders a minimal NATS config with core port, WebSocket port, and optional JetStream block.
- `internal/nats/config_test.go`
  Verifies the generated config content and omission of JetStream in the first slice.
- `internal/supervisor/commands.go`
  Defines the fixed service command set for NATS-enabled bundles.
- `internal/supervisor/commands_test.go`
  Verifies `postgres` and `nats-server` command wiring.
- `cmd/ociger-pg-launcher/main.go`
  Stays the PostgreSQL bootstrap entrypoint for non-NATS bundles and becomes the PostgreSQL child process for NATS bundles.
- `cmd/ociger-supervisor/main.go`
  Starts `ociger-pg-launcher` and `nats-server`, forwards signals, and tears down both services coherently.
- `bundles/core-pg17-micro/bundle.yaml`
  Declares the micro PostgreSQL bundle.
- `bundles/core-pg17-nats/bundle.yaml`
  Declares the stable PostgreSQL + NATS bundle.
- `bundles/core-pg17-nats-micro/bundle.yaml`
  Declares the micro PostgreSQL + NATS bundle.
- `bundles/*/.gitignore`
  Keeps generated files tracked the same way as the existing bundles.
- `scripts/build-core-pg17-micro.sh`
  Native-arch local build for `core-pg17-micro`.
- `scripts/smoke-core-pg17-micro.sh`
  PostgreSQL-only smoke for `core-pg17-micro`.
- `scripts/build-core-pg17-nats.sh`
  Native-arch local build for `core-pg17-nats`.
- `scripts/smoke-core-pg17-nats.sh`
  Smoke for PostgreSQL + NATS stable bundle.
- `scripts/build-core-pg17-nats-micro.sh`
  Native-arch local build for `core-pg17-nats-micro`.
- `scripts/smoke-core-pg17-nats-micro.sh`
  Smoke for PostgreSQL + NATS micro bundle.
- `scripts/measure-image-size.sh`
  Measures uncompressed image size and compressed archive size for README and release notes.
- `.github/workflows/core-pg17-micro-release.yml`
  Release verification and publish flow for `core-pg17-micro`.
- `.github/workflows/core-pg17-nats-release.yml`
  Release verification and publish flow for `core-pg17-nats`.
- `.github/workflows/core-pg17-nats-micro-release.yml`
  Release verification and publish flow for `core-pg17-nats-micro`.
- `README.md`
  Compact release matrix, bundle chain, launch commands, measured sizes, and future JetStream/WSS rows.

### Task 1: Add failing tests for the expanded bundle schema and rendering

**Files:**
- Create: `internal/bundle/load_test.go`
- Modify: `internal/bundle/render_test.go`
- Modify: `internal/bundle/spec.go`

- [ ] **Step 1: Write the failing schema-load test**

Create `internal/bundle/load_test.go` with a YAML fixture proving the loader can parse runtime profile, ports, and NATS service config:

```go
func TestLoadParsesRuntimeProfilePortsAndNATS(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "bundle.yaml")
	data := []byte(`
name: core-pg17-nats-micro
description: PostgreSQL 17 micro runtime with NATS
image:
  registry: ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: scratch
  runtime_profile: micro
platforms:
  - linux/amd64
ports:
  - name: postgres
    container_port: 5432
  - name: nats
    container_port: 4222
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
    jetstream: false
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-core-pg17-nats-micro-smoke/pgdata
  network: ociger-core-pg17-nats-micro-net
  container: ociger-core-pg17-nats-micro-smoke
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	spec, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if spec.Image.RuntimeProfile != "micro" {
		t.Fatalf("runtime profile = %q", spec.Image.RuntimeProfile)
	}
	if spec.Services.NATS == nil {
		t.Fatal("expected NATS service config")
	}
	if got := spec.Services.NATS.WebSocketPort; got != 9222 {
		t.Fatalf("websocket port = %d", got)
	}
}
```

- [ ] **Step 2: Add failing render tests for the micro and NATS variants**

Extend `internal/bundle/render_test.go` with three new tests that assert:

- `core-pg17-micro` uses `FROM scratch`
- `core-pg17-micro` copies only `postgres`, `initdb`, and `plpgsql.so`
- `core-pg17-nats` and `core-pg17-nats-micro` include:
  - `FROM nats:2.14.1-scratch AS nats_source`
  - `COPY --from=nats_source /nats-server /out/usr/local/bin/nats-server`
  - `EXPOSE 5432 4222 9222`
  - `COPY bundles/.../nats-server.conf /etc/nats/nats-server.conf`
  - `ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]`

Use assertions like:

```go
if !strings.Contains(df, "FROM scratch") {
	t.Fatalf("Dockerfile missing scratch final stage:\n%s", df)
}

if strings.Contains(df, "cp -a /usr/lib/postgresql/17 /out/usr/lib/postgresql/;") {
	t.Fatalf("micro Dockerfile copied full postgres tree:\n%s", df)
}

if !strings.Contains(df, "FROM nats:2.14.1-scratch AS nats_source") {
	t.Fatalf("Dockerfile missing nats source stage:\n%s", df)
}
```

- [ ] **Step 3: Extend the spec structs just enough for the tests to compile**

Add the new fields in `internal/bundle/spec.go`:

```go
type ImageSpec struct {
	Registry       string `yaml:"registry"`
	PGMajor        int    `yaml:"pg_major"`
	BaseImage      string `yaml:"base_image"`
	FinalImage     string `yaml:"final_image"`
	RuntimeProfile string `yaml:"runtime_profile"`
}

type PortSpec struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"container_port"`
}

type ServiceSpec struct {
	NATS *NATSServiceSpec `yaml:"nats,omitempty"`
}

type NATSServiceSpec struct {
	SourceImage    string `yaml:"source_image"`
	CorePort       int    `yaml:"core_port"`
	WebSocketPort  int    `yaml:"websocket_port"`
	JetStream      bool   `yaml:"jetstream"`
}
```

- [ ] **Step 4: Run the bundle tests to verify they fail**

Run:

```bash
go test ./internal/bundle
```

Expected: FAIL because the new render paths and loader behavior are not implemented yet.

- [ ] **Step 5: Commit**

```bash
git add internal/bundle/spec.go internal/bundle/load_test.go internal/bundle/render_test.go
git commit -m "test: cover micro and nats bundle rendering"
```

### Task 2: Implement the generator support for runtime profiles, ports, and NATS config assets

**Files:**
- Modify: `internal/bundle/load.go`
- Modify: `internal/bundle/render.go`
- Create: `internal/nats/config.go`
- Create: `internal/nats/config_test.go`

- [ ] **Step 1: Add the NATS config renderer with a failing test first**

Create `internal/nats/config_test.go`:

```go
func TestRenderConfigWithoutJetStream(t *testing.T) {
	cfg := Config{
		CorePort:      4222,
		WebSocketPort: 9222,
		JetStream:     false,
	}

	out := Render(cfg)

	for _, want := range []string{
		"port: 4222",
		"websocket {",
		"port: 9222",
		"no_tls: true",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("config missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "jetstream") {
		t.Fatalf("config unexpectedly enables jetstream:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the new NATS test to verify it fails**

Run:

```bash
go test ./internal/nats
```

Expected: FAIL because `internal/nats/config.go` does not exist yet.

- [ ] **Step 3: Implement the NATS config renderer**

Create `internal/nats/config.go`:

```go
package nats

import "fmt"

type Config struct {
	CorePort      int
	WebSocketPort int
	JetStream     bool
}

func Render(cfg Config) string {
	out := fmt.Sprintf("port: %d\nwebsocket {\n  port: %d\n  no_tls: true\n}\n", cfg.CorePort, cfg.WebSocketPort)
	if cfg.JetStream {
		out += "jetstream {\n  store_dir: \"/var/lib/nats\"\n}\n"
	}
	return out
}
```

- [ ] **Step 4: Extend `Render` to emit the new variants and optional NATS config**

Update `internal/bundle/render.go` so it:

- defaults `RuntimeProfile` to `stable` when empty
- renders a micro PostgreSQL source stage with selective copies
- renders an optional `nats_source` stage and `EXPOSE` line when `spec.Services.NATS != nil`
- returns an extra `nats-server.conf` asset when NATS is enabled

Use concrete snippets like:

```dockerfile
FROM nats:2.14.1-scratch AS nats_source

COPY --from=nats_source /nats-server /out/usr/local/bin/nats-server
COPY {{ .BundleDir }}/nats-server.conf /etc/nats/nats-server.conf
EXPOSE 5432 4222 9222
```

and for the micro profile:

```dockerfile
FROM {{ .Image.BaseImage }} AS postgres_source
RUN set -eux; \
  mkdir -p /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension /out/etc /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so
```

- [ ] **Step 5: Write the generated NATS config asset in `Write`**

Extend the write path so NATS-enabled bundles also get:

```go
if spec.Services.NATS != nil {
	confPath := filepath.Join(filepath.Dir(dockerfilePath), "nats-server.conf")
	if err := os.WriteFile(confPath, []byte(natsconf), 0o644); err != nil {
		return fmt.Errorf("write nats config: %w", err)
	}
}
```

- [ ] **Step 6: Run the focused tests to verify they pass**

Run:

```bash
go test ./internal/nats ./internal/bundle
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/bundle/load.go internal/bundle/render.go internal/nats/config.go internal/nats/config_test.go
git commit -m "feat: add runtime profiles and nats bundle rendering"
```

### Task 3: Add `core-pg17-micro` bundle generation, local build, smoke, and size measurement

**Files:**
- Create: `bundles/core-pg17-micro/bundle.yaml`
- Create: `bundles/core-pg17-micro/.gitignore`
- Create: `scripts/build-core-pg17-micro.sh`
- Create: `scripts/smoke-core-pg17-micro.sh`
- Create: `scripts/measure-image-size.sh`

- [ ] **Step 1: Add the micro bundle spec**

Create `bundles/core-pg17-micro/bundle.yaml`:

```yaml
name: core-pg17-micro
description: Minimal embedded PostgreSQL 17 runtime with aggressively pruned runtime files
image:
  registry: ghcr.io/sporaxis-com/ociger-core-pg17-micro
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: scratch
  runtime_profile: micro
platforms:
  - linux/amd64
  - linux/arm64
ports:
  - name: postgres
    container_port: 5432
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-core-pg17-micro-smoke/pgdata
  network: ociger-core-pg17-micro-net
  container: ociger-core-pg17-micro-smoke
```

- [ ] **Step 2: Regenerate outputs and inspect the new Dockerfile**

Run:

```bash
bash scripts/generate.sh
sed -n '1,220p' bundles/core-pg17-micro/Dockerfile
```

Expected: the file contains `FROM scratch`, the selective PostgreSQL copy list, and `ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]`.

- [ ] **Step 3: Add the size measurement helper**

Create `scripts/measure-image-size.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

IMAGE="$1"
SLUG="$2"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$ROOT/.artifacts"

UNCOMPRESSED="$(docker image inspect "$IMAGE" --format '{{.Size}}')"
ARCHIVE="$ROOT/.artifacts/${SLUG}.tar.gz"
docker save "$IMAGE" | gzip -c > "$ARCHIVE"
COMPRESSED="$(wc -c < "$ARCHIVE")"

echo "uncompressed_bytes=$UNCOMPRESSED"
echo "compressed_bytes=$COMPRESSED"
```

- [ ] **Step 4: Add the micro build and smoke scripts**

Mirror the existing core scripts with the new names and paths:

```bash
docker buildx build \
  --load \
  --platform "$platform" \
  -f bundles/core-pg17-micro/Dockerfile \
  -t ociger-core-pg17-micro:local \
  .
```

and in `scripts/smoke-core-pg17-micro.sh` reuse the current SQL proof pattern with the new `ociger-core-pg17-micro-*` names.

- [ ] **Step 5: Run the smoke script before the build to verify failure**

Run:

```bash
bash scripts/smoke-core-pg17-micro.sh
```

Expected: FAIL because `ociger-core-pg17-micro:local` does not exist yet.

- [ ] **Step 6: Build, smoke, and measure the micro image**

Run:

```bash
bash scripts/build-core-pg17-micro.sh
bash scripts/smoke-core-pg17-micro.sh
bash scripts/measure-image-size.sh ociger-core-pg17-micro:local ociger-core-pg17-micro-local
```

Expected: PASS on the smoke script and printed size lines smaller than the current released `core-pg17` numbers.

- [ ] **Step 7: Commit**

```bash
git add bundles/core-pg17-micro scripts/build-core-pg17-micro.sh scripts/smoke-core-pg17-micro.sh scripts/measure-image-size.sh
git commit -m "feat: add core pg17 micro bundle"
```

### Task 4: Add failing tests for the NATS service layer and supervisor wiring

**Files:**
- Create: `internal/supervisor/commands.go`
- Create: `internal/supervisor/commands_test.go`

- [ ] **Step 1: Write the failing supervisor command test**

Create `internal/supervisor/commands_test.go`:

```go
func TestDefaultPrograms(t *testing.T) {
	programs := DefaultPrograms()

	if len(programs) != 2 {
		t.Fatalf("program count = %d", len(programs))
	}
	if programs[0].Path != "/usr/local/bin/ociger-pg-launcher" {
		t.Fatalf("postgres path = %q", programs[0].Path)
	}
	if !reflect.DeepEqual(programs[1].Args, []string{"--config", "/etc/nats/nats-server.conf"}) {
		t.Fatalf("nats args = %#v", programs[1].Args)
	}
}
```

- [ ] **Step 2: Run the supervisor test to verify it fails**

Run:

```bash
go test ./internal/supervisor
```

Expected: FAIL because the package does not exist yet.

- [ ] **Step 3: Implement the command definition**

Create `internal/supervisor/commands.go`:

```go
package supervisor

type Program struct {
	Name string
	Path string
	Args []string
}

func DefaultPrograms() []Program {
	return []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
	}
}
```

- [ ] **Step 4: Run the supervisor test to verify it passes**

Run:

```bash
go test ./internal/supervisor
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/supervisor/commands.go internal/supervisor/commands_test.go
git commit -m "test: pin nats supervisor commands"
```

### Task 5: Implement the NATS supervisor and NATS-enabled bundle generation

**Files:**
- Create: `cmd/ociger-supervisor/main.go`
- Modify: `internal/bundle/render.go`
- Create: `bundles/core-pg17-nats/bundle.yaml`
- Create: `bundles/core-pg17-nats/.gitignore`
- Create: `bundles/core-pg17-nats-micro/bundle.yaml`
- Create: `bundles/core-pg17-nats-micro/.gitignore`

- [ ] **Step 1: Implement the static supervisor**

Create `cmd/ociger-supervisor/main.go` with the minimum process model:

```go
programs := supervisor.DefaultPrograms()
cmds := make([]*exec.Cmd, 0, len(programs))
for _, program := range programs {
	cmd := exec.Command(program.Path, program.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		log.Fatalf("start %s: %v", program.Name, err)
	}
	cmds = append(cmds, cmd)
}
```

Then:

- wait for SIGTERM/SIGINT or first child exit
- send `SIGTERM` to the sibling process groups
- wait for both children
- exit non-zero on unexpected child failure

- [ ] **Step 2: Add a supervisor build stage and NATS stage to the Dockerfile template**

Add to `internal/bundle/render.go` for NATS-enabled bundles:

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS supervisor_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-supervisor/main.go ./cmd/ociger-supervisor/main.go
COPY internal/supervisor ./internal/supervisor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-supervisor ./cmd/ociger-supervisor

FROM nats:2.14.1-scratch AS nats_source
```

and in the final stage:

```dockerfile
COPY --from=nats_source /nats-server /usr/local/bin/nats-server
COPY --from=supervisor_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor
COPY {{ .BundleDir }}/nats-server.conf /etc/nats/nats-server.conf
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
```

- [ ] **Step 3: Add the two NATS bundle specs**

Create `bundles/core-pg17-nats/bundle.yaml`:

```yaml
name: core-pg17-nats
description: Minimal PostgreSQL 17 runtime with colocated NATS core and websocket listeners
image:
  registry: ghcr.io/sporaxis-com/ociger-core-pg17-nats
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: gcr.io/distroless/base-debian12:latest
  runtime_profile: stable
platforms:
  - linux/amd64
  - linux/arm64
ports:
  - name: postgres
    container_port: 5432
  - name: nats
    container_port: 4222
  - name: nats-ws
    container_port: 9222
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
    jetstream: false
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-core-pg17-nats-smoke/pgdata
  network: ociger-core-pg17-nats-net
  container: ociger-core-pg17-nats-smoke
```

and `bundles/core-pg17-nats-micro/bundle.yaml` with `final_image: scratch` and `runtime_profile: micro`.

- [ ] **Step 4: Regenerate and run the focused tests**

Run:

```bash
bash scripts/generate.sh
go test ./internal/bundle ./internal/nats ./internal/supervisor
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ociger-supervisor/main.go bundles/core-pg17-nats bundles/core-pg17-nats-micro internal/bundle/render.go
git commit -m "feat: add nats-enabled bundle generation"
```

### Task 6: Add local build, smoke, and size measurement for the NATS variants

**Files:**
- Create: `scripts/build-core-pg17-nats.sh`
- Create: `scripts/smoke-core-pg17-nats.sh`
- Create: `scripts/build-core-pg17-nats-micro.sh`
- Create: `scripts/smoke-core-pg17-nats-micro.sh`

- [ ] **Step 1: Write the stable NATS smoke script first**

Create `scripts/smoke-core-pg17-nats.sh` by copying the current PostgreSQL smoke pattern and adding NATS listener checks:

```bash
INFO_LINE="$(
  docker run --rm --network "$NETWORK" busybox:1.37.0 \
    sh -c "nc -w 2 '$CONTAINER' 4222 < /dev/null | head -n 1"
)"
case "$INFO_LINE" in
  INFO\ *) ;;
  *)
    echo "expected NATS INFO line on 4222, got: $INFO_LINE" >&2
    exit 1
    ;;
esac

docker run --rm --network "$NETWORK" busybox:1.37.0 \
  sh -c "nc -zvw 2 '$CONTAINER' 9222"
```

Retain the PostgreSQL relation-file proof from `scripts/smoke-core-pg17.sh`.

- [ ] **Step 2: Run the stable NATS smoke before implementation to verify failure**

Run:

```bash
bash scripts/smoke-core-pg17-nats.sh
```

Expected: FAIL because `ociger-core-pg17-nats:local` does not exist yet.

- [ ] **Step 3: Add the stable NATS build script**

Create:

```bash
docker buildx build \
  --load \
  --platform "$platform" \
  -f bundles/core-pg17-nats/Dockerfile \
  -t ociger-core-pg17-nats:local \
  .
```

- [ ] **Step 4: Clone the same smoke/build pattern for the micro NATS variant**

Create:

- `scripts/build-core-pg17-nats-micro.sh`
- `scripts/smoke-core-pg17-nats-micro.sh`

with image name `ociger-core-pg17-nats-micro:local` and the corresponding `ociger-*` network/container/data-dir names.

- [ ] **Step 5: Build, smoke, and measure both NATS variants**

Run:

```bash
bash scripts/build-core-pg17-nats.sh
bash scripts/smoke-core-pg17-nats.sh
bash scripts/measure-image-size.sh ociger-core-pg17-nats:local ociger-core-pg17-nats-local

bash scripts/build-core-pg17-nats-micro.sh
bash scripts/smoke-core-pg17-nats-micro.sh
bash scripts/measure-image-size.sh ociger-core-pg17-nats-micro:local ociger-core-pg17-nats-micro-local
```

Expected:

- both smoke scripts PASS
- both images print measured size values
- `core-pg17-nats-micro` is smaller than `core-pg17-nats`

- [ ] **Step 6: Commit**

```bash
git add scripts/build-core-pg17-nats.sh scripts/smoke-core-pg17-nats.sh scripts/build-core-pg17-nats-micro.sh scripts/smoke-core-pg17-nats-micro.sh
git commit -m "feat: add local nats bundle smoke"
```

### Task 7: Add release workflows and publish the public images

**Files:**
- Create: `.github/workflows/core-pg17-micro-release.yml`
- Create: `.github/workflows/core-pg17-nats-release.yml`
- Create: `.github/workflows/core-pg17-nats-micro-release.yml`

- [ ] **Step 1: Add the micro release workflow**

Copy the current `core-pg17-release.yml` shape and target the new bundle:

```yaml
name: core-pg17-micro-release
on:
  pull_request:
  push:
    branches: [main]
    tags: [core-pg17-micro-v*]
  schedule:
    - cron: '23 4 * * *'
  workflow_dispatch:
```

Use:

- build script `bash scripts/build-core-pg17-micro.sh`
- smoke script `bash scripts/smoke-core-pg17-micro.sh`
- image `ghcr.io/${GITHUB_REPOSITORY_OWNER,,}/ociger-core-pg17-micro`

- [ ] **Step 2: Add the stable and micro NATS release workflows**

Follow the same pattern with:

- `core-pg17-nats-v*`
- `core-pg17-nats-micro-v*`

and their matching build/smoke scripts and image names.

- [ ] **Step 3: Push `main` and create the release tags**

```bash
git add .github/workflows/core-pg17-micro-release.yml .github/workflows/core-pg17-nats-release.yml .github/workflows/core-pg17-nats-micro-release.yml
git commit -m "ci: release micro and nats bundle variants"
```

- [ ] **Step 4: Push `main` and create the release tags**

Run:

```bash
git push origin main
git tag core-pg17-micro-v0.1.0
git tag core-pg17-nats-v0.1.0
git tag core-pg17-nats-micro-v0.1.0
git push origin core-pg17-micro-v0.1.0
git push origin core-pg17-nats-v0.1.0
git push origin core-pg17-nats-micro-v0.1.0
```

- [ ] **Step 5: Watch the workflows and verify publish success**

Run:

```bash
gh run watch --exit-status "$(gh run list --workflow core-pg17-micro-release.yml --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch --exit-status "$(gh run list --workflow core-pg17-nats-release.yml --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch --exit-status "$(gh run list --workflow core-pg17-nats-micro-release.yml --limit 1 --json databaseId --jq '.[0].databaseId')"
```

Expected: all three workflows finish with `conclusion: success`.

### Task 8: Verify public pulls, update the README matrix, and push the docs

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Verify anonymous pull and public smoke for all three new images**

Run:

```bash
docker logout ghcr.io >/dev/null 2>&1 || true
docker image rm -f ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.0 >/dev/null 2>&1 || true
docker image rm -f ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.0 >/dev/null 2>&1 || true
docker image rm -f ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.0 >/dev/null 2>&1 || true

docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.0
docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.0
docker buildx imagetools inspect ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.0

docker pull ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.0
docker pull ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.0
docker pull ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.0

bash scripts/smoke-core-pg17-micro.sh ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.0
bash scripts/smoke-core-pg17-nats.sh ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.0
bash scripts/smoke-core-pg17-nats-micro.sh ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.0
```

Expected: all pulls succeed anonymously and all public-image smoke tests PASS.

- [ ] **Step 2: Update the compact README matrix with measured sizes and verified behavior**

Add rows for:

- `core-pg17-micro`
- `core-pg17-nats`
- `core-pg17-nats-micro`

and planned rows for:

- `core-pg17-nats+jetstream`
- `core-pg17-nats-micro+jetstream`
- `core-pg17-nats+wss`
- `core-pg17-nats-micro+wss`

Use a compact table shape like:

```md
| Bundle | OCI image | Platforms | Size | Verified behavior |
| --- | --- | --- | --- | --- |
| `core-pg17-micro` | `ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.0` | `amd64`, `arm64` | `...` | boots, creates DB/table/row, proves relation file |
| `core-pg17-nats` | `ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.0` | `amd64`, `arm64` | `...` | PostgreSQL proof + NATS `4222` + WS `9222` live |
```

- [ ] **Step 3: Push the README update and verify the docs-only workflows**

Run:

```bash
git add README.md
git commit -m "docs: record micro and nats bundle variants"
git push origin main
gh run list --commit "$(git rev-parse HEAD)" --json workflowName,status,conclusion
```

Expected: the push reaches `origin/main` and the resulting verify workflows complete successfully.

## Self-Review Checklist

- Spec coverage:
  - `core-pg17-micro` runtime and size reduction are covered by Tasks 1-3 and 7-8.
  - reusable NATS service layer and second WebSocket port are covered by Tasks 1-2 and 4-6.
  - public multi-arch releases and README matrix updates are covered by Tasks 7-8.
- Placeholder scan:
  - no `TODO`, `TBD`, or “similar to Task N” references remain.
- Type consistency:
  - `RuntimeProfile`, `PortSpec`, `ServiceSpec`, and `NATSServiceSpec` are introduced once and reused with the same names throughout the plan.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-26-core-pg17-micro-and-nats-bundles.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
