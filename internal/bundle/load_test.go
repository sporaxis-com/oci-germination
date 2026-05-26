package bundle

import (
	"os"
	"path/filepath"
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

	if len(spec.Ports) != 2 {
		t.Fatalf("ports length = %d", len(spec.Ports))
	}

	if got := spec.Ports[0].ContainerPort; got != 5432 {
		t.Fatalf("postgres container port = %d", got)
	}

	if got := spec.Ports[1].ContainerPort; got != 4222 {
		t.Fatalf("nats container port = %d", got)
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

	if spec.Services.NATS.JetStream {
		t.Fatal("expected jetstream to be disabled")
	}
}
