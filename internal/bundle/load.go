package bundle

import (
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

	err = yaml.Unmarshal(data, &spec)
	if err != nil {
		return spec, err
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
