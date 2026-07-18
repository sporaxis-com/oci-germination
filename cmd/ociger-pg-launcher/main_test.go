package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBin_ConstructsPgBinaryPath(t *testing.T) {
	cases := map[string]string{
		"initdb":   "/usr/lib/postgresql/18/bin/initdb",
		"postgres": "/usr/lib/postgresql/18/bin/postgres",
		"pg_ctl":   "/usr/lib/postgresql/18/bin/pg_ctl",
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			if got := bin(name); got != want {
				t.Errorf("bin(%q) = %q, want %q", name, got, want)
			}
		})
	}
}

func TestGetenv_ReturnsValueWhenSet(t *testing.T) {
	t.Setenv("OCIGER_PG_LAUNCHER_TEST_KEY", "actual-value")
	if got := getenv("OCIGER_PG_LAUNCHER_TEST_KEY", "fallback"); got != "actual-value" {
		t.Errorf("getenv(set) = %q, want 'actual-value'", got)
	}
}

func TestGetenv_ReturnsFallbackWhenUnset(t *testing.T) {
	t.Setenv("OCIGER_PG_LAUNCHER_TEST_KEY", "")
	if got := getenv("OCIGER_PG_LAUNCHER_TEST_KEY", "fallback"); got != "fallback" {
		t.Errorf("getenv(empty) = %q, want 'fallback'", got)
	}
}

func TestGetenv_ReturnsFallbackForMissingKey(t *testing.T) {
	if got := getenv("OCIGER_PG_LAUNCHER_KEY_THAT_DOES_NOT_EXIST_XYZ", "default-here"); got != "default-here" {
		t.Errorf("getenv(missing) = %q, want 'default-here'", got)
	}
}

func TestAppendFile_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.conf")
	if err := os.WriteFile(path, []byte("original\n"), 0o600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	// appendFile uses log.Fatal on error, so we test the happy path only.
	// Calling on a writable existing file should succeed.
	appendFile(path, "appended\n")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := "original\nappended\n"
	if string(got) != want {
		t.Errorf("contents = %q, want %q", string(got), want)
	}
}

func TestAppendFile_MultipleAppendsAccumulate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.conf")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	appendFile(path, "line1\n")
	appendFile(path, "line2\n")
	appendFile(path, "line3\n")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := "line1\nline2\nline3\n"
	if string(got) != want {
		t.Errorf("contents = %q, want %q", string(got), want)
	}
}

func TestMust_NilErrorDoesNotPanic(t *testing.T) {
	// must(nil) should be a no-op
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("must(nil) panicked: %v", r)
		}
	}()
	must(nil)
}

// Note: must(err) for non-nil err calls log.Fatal which terminates the
// process. Can't test in-process without subprocess gymnastics. The
// must() function is 1 LOC of guard logic — leaving it untested is fine.
