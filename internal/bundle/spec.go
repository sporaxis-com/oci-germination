package bundle

type Spec struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	BundleDir   string      `yaml:"-"`
	Image       ImageSpec   `yaml:"image"`
	Platforms   []string    `yaml:"platforms"`
	Ports       []PortSpec  `yaml:"ports"`
	Services    ServiceSpec `yaml:"services"`
	Local       LocalSpec   `yaml:"local"`
}

type ImageSpec struct {
	Registry       string `yaml:"registry"`
	PGMajor        int    `yaml:"pg_major"`
	BaseImage      string `yaml:"base_image"`
	FinalImage     string `yaml:"final_image"`
	RuntimeProfile string `yaml:"runtime_profile"`
}

type PortSpec struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"container_port"`
}

type ServiceSpec struct {
	NATS *NATSServiceSpec `yaml:"nats,omitempty"`
}

type NATSServiceSpec struct {
	SourceImage   string `yaml:"source_image"`
	CorePort      int    `yaml:"core_port"`
	WebSocketPort int    `yaml:"websocket_port"`
	JetStream     bool   `yaml:"jetstream"`
}

type LocalSpec struct {
	Prefix    string `yaml:"prefix"`
	DataDir   string `yaml:"data_dir"`
	Network   string `yaml:"network"`
	Container string `yaml:"container"`
}
