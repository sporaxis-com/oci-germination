package supervisor

import (
	"reflect"
	"testing"
)

func TestDefaultPrograms(t *testing.T) {
	programs := DefaultPrograms()

	if len(programs) != 2 {
		t.Fatalf("program count = %d", len(programs))
	}
	if programs[0].Path != "/usr/local/bin/ociger-pg-launcher" {
		t.Fatalf("postgres path = %q", programs[0].Path)
	}
	if !reflect.DeepEqual(programs[1].Args, []string{"--config", "/etc/nats/nats-server.conf"}) {
		t.Fatalf("nats args = %#v", programs[1].Args)
	}
}
