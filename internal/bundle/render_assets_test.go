package bundle

import (
	"strings"
	"testing"
)

func TestRenderAssetsIncludesNATSConfigWhenEnabled(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17-nats",
		Description: "Minimal embedded PostgreSQL 17 runtime with NATS",
		BundleDir:   "bundles/core-pg17-nats",
		Image: ImageSpec{
			Registry:   "ghcr.io/sporaxis-com/ociger-core-pg17-nats",
			PGMajor:    17,
			BaseImage:  "postgres:17-bookworm",
			FinalImage: "gcr.io/distroless/base-debian12:latest",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Ports: []PortSpec{
			{Name: "postgres", ContainerPort: 5432},
			{Name: "nats", ContainerPort: 4222},
			{Name: "nats-websocket", ContainerPort: 9222},
		},
		Services: ServiceSpec{
			NATS: &NATSServiceSpec{
				SourceImage:   "nats:2.14.1-scratch",
				CorePort:      4222,
				WebSocketPort: 9222,
				JetStream:     false,
			},
		},
	}

	assets, err := RenderAssets(spec)
	if err != nil {
		t.Fatalf("RenderAssets returned error: %v", err)
	}

	if !strings.Contains(assets.Dockerfile, "COPY bundles/core-pg17-nats/nats-server.conf /etc/nats/nats-server.conf") {
		t.Fatalf("dockerfile missing nats config copy:\n%s", assets.Dockerfile)
	}
	if assets.Bake == "" {
		t.Fatal("expected bake output")
	}
	for _, want := range []string{
		"port: 4222",
		"websocket {",
		"port: 9222",
		"no_tls: true",
	} {
		if !strings.Contains(assets.NATSConfig, want) {
			t.Fatalf("nats config missing %q:\n%s", want, assets.NATSConfig)
		}
	}
}
