package supervisor

import "os"

type Program struct {
	Name string
	Path string
	Args []string
}

// DefaultPrograms returns the program list for the current supervisor profile.
// Selected via OCIGER_SUPERVISOR_PROFILE env (default: postgres + nats only).
func DefaultPrograms() []Program {
	switch os.Getenv("OCIGER_SUPERVISOR_PROFILE") {
	case "static":
		return staticProfile()
	default:
		return defaultProfile()
	}
}

func defaultProfile() []Program {
	return []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
	}
}

// staticProfile adds ociger-static-server alongside postgres + nats.
// Used by bundles that serve static web content (e.g. CK.Lib.Js) without
// a Python/FastAPI runtime.
func staticProfile() []Program {
	return []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
		{Name: "static", Path: "/usr/local/bin/ociger-static-server", Args: []string{"-root", "/app", "-addr", ":8000"}},
	}
}
