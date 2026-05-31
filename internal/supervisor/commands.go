package supervisor

import "os"

type Program struct {
	Name string
	Path string
	Args []string
}

// DefaultPrograms returns the program list for the current supervisor profile.
// Selected via OCIGER_SUPERVISOR_PROFILE env (default: postgres + nats only).
//
// The legacy "fastapi" profile has been removed: ck-allinone moved to s6-overlay
// for supervision (no longer uses ociger-supervisor at all), and prod ck-allinone
// no longer bundles a Python/FastAPI runtime. Any Python-bearing benchmark
// container lives as a separate bundle (e.g. ociger-pgck-bench) and uses its
// own supervision, not this binary.
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
// Used by bundle-pg17-pgrdf-pgck-static-cklib (which still uses ociger-supervisor).
// Going forward, prod bundles should prefer s6-overlay + busybox httpd; this
// profile stays for the static-cklib bundle until it migrates to the same shape.
func staticProfile() []Program {
	return []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
		{Name: "static", Path: "/usr/local/bin/ociger-static-server", Args: []string{"-root", "/app", "-addr", ":8000"}},
	}
}
