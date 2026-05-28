package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", "/app", "root directory containing static mounts (e.g. /app/cklib)")
	addr := flag.String("addr", ":8000", "listen address")
	flag.Parse()

	mux, mounted, err := buildMux(*root, log.Default())
	if err != nil {
		log.Fatalf("build mux: %v", err)
	}
	if mounted == 0 {
		log.Printf("warning: no subdirectories found under %s; only /healthz available", *root)
	}

	log.Printf("ociger-static-server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// buildMux scans root for subdirectories and mounts each at /<name>/.
// Also registers /healthz. Returns the mux, the number of mounts, and
// any error reading root. Exposed for testing.
func buildMux(root string, logger *log.Logger) (*http.ServeMux, int, error) {
	mux := http.NewServeMux()

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0, fmt.Errorf("read root %s: %w", root, err)
	}

	mounted := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mountPath := "/" + e.Name() + "/"
		dirPath := filepath.Join(root, e.Name())
		mux.Handle(mountPath, http.StripPrefix(mountPath, http.FileServer(http.Dir(dirPath))))
		if logger != nil {
			logger.Printf("mount %s → %s", mountPath, dirPath)
		}
		mounted++
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return mux, mounted, nil
}
