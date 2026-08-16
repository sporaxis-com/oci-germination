package bundle

type Spec struct {
	Name        string `yaml:"name"`
	SpecVersion string `yaml:"spec_version,omitempty"`
	Description string `yaml:"description"`
	BundleDir   string `yaml:"-"`
	SkipRender  bool   `yaml:"skip_render,omitempty"`
	// Role + NeverProd surface the ck.bundle.role + ck.bundle.never-prod
	// manifest labels emitted in the bundle's Dockerfile final stage.
	// Valid Role values: "prod" | "devel" | "bench".
	Role       string        `yaml:"role,omitempty"`
	NeverProd  bool          `yaml:"never_prod,omitempty"`
	Image      ImageSpec     `yaml:"image"`
	Extensions ExtensionSpec `yaml:"extensions"`
	Platforms  []string      `yaml:"platforms"`
	Ports      []PortSpec    `yaml:"ports"`
	Services   ServiceSpec   `yaml:"services"`
	Local      LocalSpec     `yaml:"local"`

	// Version is the bundle's own release tag (e.g. "v0.7.32"). Present on
	// hand-maintained bundles; the rendered ones take it from the release tag.
	Version string `yaml:"version,omitempty"`

	// Components, Supervisor, StaticWeb, Defaults and UsageExample were carried
	// in bundle.yaml for several releases while the loader used a non-strict
	// Unmarshal, so they parsed into nothing and validated nothing — a pin that
	// read as authoritative and was never compared to anything. They are modelled
	// here so the strict loader (load.go) can bind them.
	Components   map[string]ComponentSpec `yaml:"components,omitempty"`
	Supervisor   SupervisorSpec           `yaml:"supervisor,omitempty"`
	StaticWeb    []StaticWebSpec          `yaml:"static_web,omitempty"`
	Defaults     map[string]string        `yaml:"defaults,omitempty"`
	UsageExample string                   `yaml:"usage_example,omitempty"`

	// RuntimeInputs declares what the OPERATOR delivers at container start.
	// Nothing declared here is baked into the image and nothing here is fetched
	// by the container — see RuntimeInputSpec.
	RuntimeInputs map[string]RuntimeInputSpec `yaml:"runtime_inputs,omitempty"`
}

// ComponentSpec is one non-extension part of a bundle: a supervised binary, a
// static asset set, or an upstream image whose content is layered in.
type ComponentSpec struct {
	Version     string `yaml:"version,omitempty"`
	Source      string `yaml:"source,omitempty"`
	Description string `yaml:"description,omitempty"`
	Role        string `yaml:"role,omitempty"`
	MountPath   string `yaml:"mount_path,omitempty"`
	Port        int    `yaml:"port,omitempty"`
	CorePort    int    `yaml:"core_port,omitempty"`
	WSSPort     int    `yaml:"wss_port,omitempty"`
	JetStream   bool   `yaml:"jetstream,omitempty"`
	RunsAs      string `yaml:"runs_as,omitempty"`
	Security    string `yaml:"security,omitempty"`
}

type SupervisorSpec struct {
	Binary      string `yaml:"binary,omitempty"`
	Profile     string `yaml:"profile,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type StaticWebSpec struct {
	SourceImage string `yaml:"source_image"`
	Route       string `yaml:"route"`
}

// RuntimeInputSpec is a value the operator supplies AT CONTAINER START.
//
// Two invariants make this block worth declaring rather than documenting:
//
//	NeverBaked — the value is not in the image, so the published artifact is the
//	  same bytes for every deployment and carries no deployment's secrets.
//	NoEgress   — the container does not fetch it. A bundle that reaches out at
//	  boot puts DNS and a third party into its own start path, and fails in a
//	  way that looks like the bundle is broken.
//
// Form distinguishes a scalar from a document: `pgck.oidc_jwks` must carry the
// JWKS JSON, and supplying the URL instead is the failure og#29 records — it is
// accepted everywhere, verifies nothing, and leaves every connection anonymous.
type RuntimeInputSpec struct {
	Required    bool     `yaml:"required"`
	Form        string   `yaml:"form"`                 // "scalar" | "document"
	MediaType   string   `yaml:"media_type,omitempty"` // when Form is "document"
	Delivery    []string `yaml:"delivery,omitempty"`   // "file" | "env", in precedence order
	Env         string   `yaml:"env,omitempty"`        // the env var carrying the value
	FileEnv     string   `yaml:"file_env,omitempty"`   // the env var carrying a path to it
	ConsumedBy  string   `yaml:"consumed_by"`          // which in-tree binary reads it
	PlacedAt    string   `yaml:"placed_at,omitempty"`  // where it lands in the container
	NeverBaked  bool     `yaml:"never_baked"`
	NoEgress    bool     `yaml:"no_egress,omitempty"`
	Secret      bool     `yaml:"secret,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Gate        string   `yaml:"gate,omitempty"` // how a third party verifies it landed
}

type ImageSpec struct {
	Registry  string `yaml:"registry"`
	PGMajor   int    `yaml:"pg_major"`
	BaseImage string `yaml:"base_image"`
	// BaseImageVersion is the tag the bundle builds FROM. It decides which base
	// the image inherits and it was unmodelled until the strict loader landed —
	// so the single most load-bearing pin in the file bound to nothing.
	BaseImageVersion string `yaml:"base_image_version,omitempty"`
	FinalImage       string `yaml:"final_image"`
	RuntimeProfile   string `yaml:"runtime_profile"`
}

type PortSpec struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"container_port"`
}

type ExtensionSpec struct {
	PGRDF    *PGRDFExtensionSpec `yaml:"pgrdf,omitempty"`
	PGCK     *PGCKExtensionSpec  `yaml:"pgck,omitempty"`
	PGCrypto *PGCryptoSpec       `yaml:"pgcrypto,omitempty"`
}

// PGCryptoSpec is the in-tree PostgreSQL extension pgCK depends on. Version is
// "builtin" — it ships with the server rather than being pulled as an artifact.
type PGCryptoSpec struct {
	Version   string `yaml:"version"`
	Source    string `yaml:"source,omitempty"`
	Bootstrap string `yaml:"bootstrap,omitempty"`
}

type PGRDFExtensionSpec struct {
	Version string `yaml:"version"`
	Source  string `yaml:"source,omitempty"`
	// Bootstrap is the CREATE EXTENSION statement first boot runs.
	Bootstrap string `yaml:"bootstrap,omitempty"`
}

type PGCKExtensionSpec struct {
	Version   string `yaml:"version"`
	Source    string `yaml:"source,omitempty"`
	Bootstrap string `yaml:"bootstrap,omitempty"`
	// Contract records the role floor the extension establishes — which roles
	// CREATE EXTENSION creates and what each may reach.
	Contract string `yaml:"contract,omitempty"`
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
