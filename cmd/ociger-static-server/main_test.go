package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMux_MountsSubdirectoriesAndServesFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "cklib"))
	mustMkdir(t, filepath.Join(root, "docs"))
	mustWrite(t, filepath.Join(root, "cklib", "ck-client.js"), "console.log('cklib');")
	mustWrite(t, filepath.Join(root, "docs", "index.html"), "<h1>docs</h1>")
	// Files at root should be ignored (only directories mount).
	mustWrite(t, filepath.Join(root, "README.md"), "# readme")

	mux, mounted, err := buildMux(root, silentLogger())
	if err != nil {
		t.Fatalf("buildMux: %v", err)
	}
	if mounted != 2 {
		t.Fatalf("mounted = %d, want 2", mounted)
	}

	tests := []struct {
		name     string
		path     string
		wantCode int
		wantBody string
	}{
		{"cklib file", "/cklib/ck-client.js", 200, "console.log('cklib');"},
		// /docs/index.html → 301 to /docs/ per http.FileServer convention; test the canonical form
		{"docs index", "/docs/", 200, "<h1>docs</h1>"},
		{"docs index.html redirects", "/docs/index.html", 301, ""},
		{"missing file", "/cklib/missing.js", 404, ""},
		{"healthz", "/healthz", 200, "ok"},
		{"unknown root", "/elsewhere/", 404, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body = %q, want substring %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestBuildMux_EmptyRoot(t *testing.T) {
	root := t.TempDir()
	mux, mounted, err := buildMux(root, silentLogger())
	if err != nil {
		t.Fatalf("buildMux: %v", err)
	}
	if mounted != 0 {
		t.Fatalf("mounted = %d, want 0", mounted)
	}
	// /healthz must still work even with no mounts.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("healthz on empty root: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestBuildMux_MissingRootReturnsError(t *testing.T) {
	_, _, err := buildMux("/nonexistent/path/that/should/not/exist", silentLogger())
	if err == nil {
		t.Fatal("buildMux on missing root should return error")
	}
	if !strings.Contains(err.Error(), "read root") {
		t.Errorf("error message should mention 'read root', got: %v", err)
	}
}

func TestBuildMux_PathTraversalBlocked(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "cklib"))
	mustWrite(t, filepath.Join(root, "cklib", "ok.txt"), "ok")
	// Sensitive file at root, NOT under any mount.
	mustWrite(t, filepath.Join(root, "secret.txt"), "do not serve")

	mux, _, err := buildMux(root, silentLogger())
	if err != nil {
		t.Fatalf("buildMux: %v", err)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/cklib/../secret.txt", nil))
	// Go's http.FileServer clean()s paths; /cklib/../secret.txt becomes /secret.txt
	// which is not under any mount → 404.
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMovedPermanently {
		t.Errorf("path traversal attempt: code = %d, want 404 or 301", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "do not serve") {
		t.Errorf("path traversal LEAKED secret file content")
	}
}

func TestBuildMux_NilLoggerDoesNotPanic(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "x"))
	_, mounted, err := buildMux(root, nil)
	if err != nil {
		t.Fatalf("buildMux with nil logger: %v", err)
	}
	if mounted != 1 {
		t.Fatalf("mounted = %d, want 1", mounted)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}
