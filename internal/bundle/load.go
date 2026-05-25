package bundle

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Spec, error) {
	var spec Spec

	data, err := os.ReadFile(path)
	if err != nil {
		return spec, err
	}

	err = yaml.Unmarshal(data, &spec)
	return spec, err
}
