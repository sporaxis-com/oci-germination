package supervisor

import (
	"reflect"
	"testing"
)

func TestDefaultPrograms(t *testing.T) {
	want := []Program{
		{Name: "postgres", Path: "/usr/local/bin/ociger-pg-launcher"},
		{Name: "nats", Path: "/usr/local/bin/nats-server", Args: []string{"--config", "/etc/nats/nats-server.conf"}},
	}

	if programs := DefaultPrograms(); !reflect.DeepEqual(programs, want) {
		t.Fatalf("DefaultPrograms() = %#v, want %#v", programs, want)
	}
}
