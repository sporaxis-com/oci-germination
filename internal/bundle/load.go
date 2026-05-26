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
	spec.BundleDir = filepath.ToSlash(filepath.Dir(path))
	return spec, err
}
