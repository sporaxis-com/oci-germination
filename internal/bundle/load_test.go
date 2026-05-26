package bundle

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
extensions:
  pgrdf:
    version: 0.5.1
  pgck:
    version: 0.1.2
platforms:
  - linux/amd64
ports:
  - name: postgres
    container_port: 5432
  - name: nats
    container_port: 4222
  - name: nats-websocket
    container_port: 9222
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
    jetstream: true
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

	if spec.Extensions.PGRDF == nil || spec.Extensions.PGRDF.Version != "0.5.1" {
		t.Fatalf("pgrdf extension = %#v", spec.Extensions.PGRDF)
	}

	if spec.Extensions.PGCK == nil || spec.Extensions.PGCK.Version != "0.1.2" {
		t.Fatalf("pgck extension = %#v", spec.Extensions.PGCK)
	}

	gotPorts := make(map[string]int, len(spec.Ports))
	for _, port := range spec.Ports {
		gotPorts[port.Name] = port.ContainerPort
	}

	wantPorts := map[string]int{
		"postgres":       5432,
		"nats":           4222,
		"nats-websocket": 9222,
	}
	if len(gotPorts) != len(wantPorts) {
		t.Fatalf("ports length = %d", len(gotPorts))
	}
	for name, want := range wantPorts {
		if got := gotPorts[name]; got != want {
			t.Fatalf("port %q = %d", name, got)
		}
	}

	if spec.Services.NATS == nil {
		t.Fatal("expected NATS service config")
	}

	if got := spec.Services.NATS.SourceImage; got != "nats:2.14.1-scratch" {
		t.Fatalf("source image = %q", got)
	}

	if got := spec.Services.NATS.CorePort; got != 4222 {
		t.Fatalf("core port = %d", got)
	}

	if got := spec.Services.NATS.WebSocketPort; got != 9222 {
		t.Fatalf("websocket port = %d", got)
	}

	if !spec.Services.NATS.JetStream {
		t.Fatal("expected jetstream to be enabled")
	}
}

func TestLoadRejectsUnknownRuntimeProfile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "bundle.yaml")
	data := []byte(`
name: core-pg17-weird
description: PostgreSQL 17 runtime with unsupported profile
image:
  registry: ghcr.io/sporaxis-com/ociger-core-pg17-weird
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: scratch
  runtime_profile: tiny
platforms:
  - linux/amd64
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-core-pg17-weird-smoke/pgdata
  network: ociger-core-pg17-weird-net
  container: ociger-core-pg17-weird-smoke
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error for invalid runtime profile")
	}
	if !strings.Contains(err.Error(), "runtime profile") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadNormalizesAbsoluteBundleDirToRepoRelativePath(t *testing.T) {
	repoRoot := testRepoRoot(t)
	bundlePath, bundleDir := writeTestBundle(t, repoRoot, "absolute-load-", []byte(`
name: core-pg17-nats-absolute
description: PostgreSQL 17 runtime with NATS loaded via absolute path
image:
  registry: ghcr.io/sporaxis-com/ociger-core-pg17-nats-absolute
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: scratch
platforms:
  - linux/amd64
ports:
  - name: postgres
    container_port: 5432
  - name: nats
    container_port: 4222
  - name: nats-websocket
    container_port: 9222
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
    jetstream: true
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-core-pg17-nats-absolute/pgdata
  network: ociger-core-pg17-nats-absolute-net
  container: ociger-core-pg17-nats-absolute
`))

	withWorkingDir(t, repoRoot, func() {
		spec, err := Load(bundlePath)
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}

		want := filepath.ToSlash(filepath.Join("bundles", filepath.Base(bundleDir)))
		if spec.BundleDir != want {
			t.Fatalf("BundleDir = %q, want %q", spec.BundleDir, want)
		}
		if filepath.IsAbs(spec.BundleDir) {
			t.Fatalf("BundleDir should be relative, got %q", spec.BundleDir)
		}
		if strings.HasPrefix(spec.BundleDir, "..") {
			t.Fatalf("BundleDir should stay inside the build context, got %q", spec.BundleDir)
		}
	})
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		t.Fatalf("resolve repo root %q: %v", repoRoot, err)
	}

	return repoRoot
}

func writeTestBundle(t *testing.T, repoRoot string, pattern string, data []byte) (string, string) {
	t.Helper()

	bundlesDir := filepath.Join(repoRoot, "bundles")
	bundleDir, err := os.MkdirTemp(bundlesDir, pattern)
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(bundleDir)
	})

	bundlePath := filepath.Join(bundleDir, "bundle.yaml")
	if err := os.WriteFile(bundlePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return bundlePath, bundleDir
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory to %q: %v", wd, err)
		}
	})

	fn()
}
