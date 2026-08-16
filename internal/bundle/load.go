package bundle

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Spec, error) {
	var spec Spec

	data, err := os.ReadFile(path)
	if err != nil {
		return spec, err
	}

	// STRICT. A plain Unmarshal silently discards any key the Spec does not
	// model, so a block could sit in bundle.yaml for releases reading like a
	// contract while binding to nothing — `components.cklib.version` was exactly
	// that. KnownFields turns an unmodelled key into a build failure naming it,
	// which is the only version of this file that can be trusted as a source.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil && err != io.EOF {
		return spec, fmt.Errorf("%s: %w", path, err)
	}
	spec, err = normalizeSpec(spec)
	if err != nil {
		return spec, err
	}

	bundleDir, err := bundleDirForPath(path)
	if err != nil {
		return spec, err
	}
	spec.BundleDir = bundleDir
	return spec, nil
}

func bundleDirForPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	relDir, err := filepath.Rel(wd, filepath.Dir(absPath))
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(relDir), nil
}
