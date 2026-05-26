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
