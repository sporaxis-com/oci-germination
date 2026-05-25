package launcher

import (
	"reflect"
	"testing"
)

func TestPostgresArgsDefaultToPlainDataDirLaunch(t *testing.T) {
	args := PostgresArgs("/var/lib/postgresql/data", "")

	want := []string{
		"/usr/lib/postgresql/17/bin/postgres",
		"-D", "/var/lib/postgresql/data",
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestPostgresArgsIncludeSharedPreloadLibrariesWhenRequested(t *testing.T) {
	args := PostgresArgs("/var/lib/postgresql/data", "pgck")

	want := []string{
		"/usr/lib/postgresql/17/bin/postgres",
		"-D", "/var/lib/postgresql/data",
		"-c", "shared_preload_libraries=pgck",
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: %#v", args)
	}
}
