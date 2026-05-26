package nats

import (
	"strings"
	"testing"
)

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
