package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/sporaxis-com/oci-germination/internal/bundle"
)

func main() {
	specPath := flag.String("bundle", "", "Path to bundle.yaml")
	flag.Parse()

	if *specPath == "" {
		log.Fatal("--bundle is required")
	}

	spec, err := bundle.Load(*specPath)
	if err != nil {
		log.Fatal(err)
	}

	if spec.SkipRender {
		log.Printf("skip_render set; leaving %s untouched", *specPath)
		return
	}

	dir := filepath.Dir(*specPath)
	if err := bundle.Write(spec, filepath.Join(dir, "Dockerfile"), filepath.Join(dir, "docker-bake.hcl")); err != nil {
		log.Fatal(err)
	}
}
