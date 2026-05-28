package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_CoreBundleProducesDockerfileAndBake(t *testing.T) {
	repoRoot := findRepoRoot(t)
	bundlePath := filepath.Join(repoRoot, "bundles/core-pg17/bundle.yaml")

	out := t.TempDir()
	dockerfilePath := filepath.Join(out, "Dockerfile")
	bakePath := filepath.Join(out, "docker-bake.hcl")

	withWorkingDir(t, repoRoot, func() {
		spec, err := Load(bundlePath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := Write(spec, dockerfilePath, bakePath); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})

	if data, err := os.ReadFile(dockerfilePath); err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	} else if !strings.Contains(string(data), "FROM") {
		t.Errorf("Dockerfile content missing FROM directive: %q", string(data))
	}

	if data, err := os.ReadFile(bakePath); err != nil {
		t.Fatalf("read bake file: %v", err)
	} else if len(data) == 0 {
		t.Errorf("bake file is empty")
	}
}

func TestWrite_NATSBundleAlsoWritesConfig(t *testing.T) {
	repoRoot := findRepoRoot(t)
	bundlePath := filepath.Join(repoRoot, "bundles/core-pg17-nats/bundle.yaml")

	out := t.TempDir()
	dockerfilePath := filepath.Join(out, "Dockerfile")
	bakePath := filepath.Join(out, "docker-bake.hcl")
	natsConfPath := filepath.Join(out, "nats-server.conf")

	withWorkingDir(t, repoRoot, func() {
		spec, err := Load(bundlePath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := Write(spec, dockerfilePath, bakePath); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})

	if _, err := os.Stat(dockerfilePath); err != nil {
		t.Errorf("Dockerfile not written: %v", err)
	}
	if _, err := os.Stat(bakePath); err != nil {
		t.Errorf("bake file not written: %v", err)
	}
	if _, err := os.Stat(natsConfPath); err != nil {
		t.Errorf("nats-server.conf not written (NATS bundle should produce one): %v", err)
	}

	conf, err := os.ReadFile(natsConfPath)
	if err != nil {
		t.Fatalf("read nats conf: %v", err)
	}
	if !strings.Contains(string(conf), "port:") && !strings.Contains(string(conf), "4222") {
		t.Errorf("nats conf missing port directive: %q", string(conf))
	}
}

func TestWrite_CreatesMissingParentDirs(t *testing.T) {
	repoRoot := findRepoRoot(t)
	bundlePath := filepath.Join(repoRoot, "bundles/core-pg17/bundle.yaml")

	out := t.TempDir()
	// Deep path under tempdir — parent dirs don't exist yet
	deepDir := filepath.Join(out, "a", "b", "c")
	dockerfilePath := filepath.Join(deepDir, "Dockerfile")
	bakePath := filepath.Join(deepDir, "docker-bake.hcl")

	withWorkingDir(t, repoRoot, func() {
		spec, err := Load(bundlePath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := Write(spec, dockerfilePath, bakePath); err != nil {
			t.Fatalf("Write should create missing dirs: %v", err)
		}
	})

	if _, err := os.Stat(dockerfilePath); err != nil {
		t.Errorf("Dockerfile not created in deep path: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// internal/bundle is two levels deep
	root := filepath.Join(wd, "..", "..")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	return abs
}
