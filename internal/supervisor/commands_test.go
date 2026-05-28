package supervisor

import (
	"reflect"
	"testing"
)

func TestDefaultPrograms_Default(t *testing.T) {
	t.Setenv("OCIGER_SUPERVISOR_PROFILE", "")
	want := []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
	}

	if programs := DefaultPrograms(); !reflect.DeepEqual(programs, want) {
		t.Fatalf("DefaultPrograms() = %#v, want %#v", programs, want)
	}
}

func TestDefaultPrograms_StaticProfile(t *testing.T) {
	t.Setenv("OCIGER_SUPERVISOR_PROFILE", "static")
	want := []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
		{Name: "static", Path: "/usr/local/bin/ociger-static-server", Args: []string{"-root", "/app", "-addr", ":8000"}},
	}

	if programs := DefaultPrograms(); !reflect.DeepEqual(programs, want) {
		t.Fatalf("DefaultPrograms(static) = %#v, want %#v", programs, want)
	}
}
