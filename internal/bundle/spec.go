package bundle

type Spec struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	BundleDir   string    `yaml:"-"`
	Image       ImageSpec `yaml:"image"`
	Platforms   []string  `yaml:"platforms"`
	Local       LocalSpec `yaml:"local"`
}

type ImageSpec struct {
	Registry   string `yaml:"registry"`
	PGMajor    int    `yaml:"pg_major"`
	BaseImage  string `yaml:"base_image"`
	FinalImage string `yaml:"final_image"`
}

type LocalSpec struct {
	Prefix    string `yaml:"prefix"`
	DataDir   string `yaml:"data_dir"`
	Network   string `yaml:"network"`
	Container string `yaml:"container"`
}
