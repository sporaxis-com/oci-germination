package bundle

type Spec struct {
	Name        string        `yaml:"name"`
	SpecVersion string        `yaml:"spec_version,omitempty"`
	Description string        `yaml:"description"`
	BundleDir   string        `yaml:"-"`
	SkipRender  bool          `yaml:"skip_render,omitempty"`
	// Role + NeverProd surface SPEC.OCI.BUNDLE.v0.4 §2.4 manifest labels
	// (ck.bundle.role + ck.bundle.never-prod) and §3 bundle.yaml fields.
	// Valid Role values: "prod" | "devel" | "bench".
	Role       string        `yaml:"role,omitempty"`
	NeverProd  bool          `yaml:"never_prod,omitempty"`
	Image      ImageSpec     `yaml:"image"`
	Extensions ExtensionSpec `yaml:"extensions"`
	Platforms  []string      `yaml:"platforms"`
	Ports      []PortSpec    `yaml:"ports"`
	Services   ServiceSpec   `yaml:"services"`
	Local      LocalSpec     `yaml:"local"`
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

type ExtensionSpec struct {
	PGRDF *PGRDFExtensionSpec `yaml:"pgrdf,omitempty"`
	PGCK  *PGCKExtensionSpec  `yaml:"pgck,omitempty"`
}

type PGRDFExtensionSpec struct {
	Version string `yaml:"version"`
}

type PGCKExtensionSpec struct {
	Version string `yaml:"version"`
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
