package bundle

import (
	"path/filepath"
	"strings"
	"testing"
)

func assertMicroRuntimeContract(t *testing.T, df string) {
	t.Helper()

	if !strings.Contains(df, "FROM scratch") {
		t.Fatalf("Dockerfile missing scratch final stage:\n%s", df)
	}

	for _, want := range []string{
		"/usr/lib/postgresql/17/bin/postgres /out/usr/lib/postgresql/17/bin/postgres",
		"/usr/lib/postgresql/17/bin/initdb /out/usr/lib/postgresql/17/bin/initdb",
		"/usr/lib/postgresql/17/lib/dict_snowball.so /out/usr/lib/postgresql/17/lib/dict_snowball.so",
		"/usr/lib/postgresql/17/lib/plpgsql.so /out/usr/lib/postgresql/17/lib/plpgsql.so",
		"/usr/share/postgresql/17/catalog_version /out/usr/share/postgresql/17/catalog_version",
		"/usr/share/postgresql/17/errcodes.txt /out/usr/share/postgresql/17/errcodes.txt",
		"/usr/share/postgresql/17/postgresql.conf.sample /out/usr/share/postgresql/17/postgresql.conf.sample",
		"/usr/share/postgresql/17/pg_service.conf.sample /out/usr/share/postgresql/17/pg_service.conf.sample",
		"/usr/share/postgresql/17/psqlrc.sample /out/usr/share/postgresql/17/psqlrc.sample",
		"/usr/share/postgresql/17/snowball_create.sql /out/usr/share/postgresql/17/snowball_create.sql",
		"/usr/share/postgresql/17/sql_features.txt /out/usr/share/postgresql/17/sql_features.txt",
	} {
		if !strings.Contains(df, want) {
			t.Fatalf("micro Dockerfile missing selective artifact copy %q:\n%s", want, df)
		}
	}

	if strings.Contains(df, "cp -a /usr/lib/postgresql/17 /out/usr/lib/postgresql/;") {
		t.Fatalf("micro Dockerfile copied full postgres tree:\n%s", df)
	}

	if strings.Contains(df, "cp -a /usr/share/postgresql/17 /out/usr/share/postgresql/;") {
		t.Fatalf("micro Dockerfile copied full postgres share tree:\n%s", df)
	}

	if !strings.Contains(df, "tr -s '[:space:]' '\\n'") {
		t.Fatalf("micro Dockerfile missing whitespace-safe ldd parsing for dynamic loader copy:\n%s", df)
	}
}

func TestRenderCoreBundle(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17",
		Description: "Minimal embedded PostgreSQL 17 runtime",
		BundleDir:   "bundles/core-pg17",
		Image: ImageSpec{
			Registry:   "ghcr.io/sporaxis-com/ociger-core-pg17-min",
			PGMajor:    17,
			BaseImage:  "postgres:17-bookworm",
			FinalImage: "gcr.io/distroless/base-debian12:latest",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Local: LocalSpec{
			Prefix:    "ociger-",
			DataDir:   ".artifacts/ociger-core-pg17-smoke/pgdata",
			Network:   "ociger-core-pg17-net",
			Container: "ociger-core-pg17-smoke",
		},
	}

	df, bake, err := Render(spec)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	if !strings.Contains(df, "FROM postgres:17-bookworm AS postgres_source") {
		t.Fatalf("Dockerfile missing postgres source stage:\n%s", df)
	}

	if !strings.Contains(df, "COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher") {
		t.Fatalf("Dockerfile missing launcher copy:\n%s", df)
	}

	if !strings.Contains(df, "COPY go.sum ./") {
		t.Fatalf("Dockerfile missing go.sum copy:\n%s", df)
	}

	if !strings.Contains(df, "COPY internal/launcher ./internal/launcher") {
		t.Fatalf("Dockerfile missing internal launcher copy:\n%s", df)
	}

	if !strings.Contains(df, "cp -L /bin/sh /out/bin/sh;") {
		t.Fatalf("Dockerfile missing dereferenced /bin/sh copy for initdb:\n%s", df)
	}

	if !strings.Contains(df, "cp -L /usr/share/postgresql/postgresql.conf.sample /out/usr/share/postgresql/postgresql.conf.sample;") {
		t.Fatalf("Dockerfile missing root postgresql.conf.sample copy for initdb:\n%s", df)
	}

	if !strings.Contains(bake, "linux/amd64") || !strings.Contains(bake, "linux/arm64") {
		t.Fatalf("Bake output missing platforms:\n%s", bake)
	}

	if !strings.Contains(bake, `dockerfile = "bundles/core-pg17/Dockerfile"`) {
		t.Fatalf("Bake output missing core Dockerfile path:\n%s", bake)
	}
}

func TestRenderPGRDFBundle(t *testing.T) {
	spec := Spec{
		Name:        "bundle-pg17-pgrdf",
		Description: "PostgreSQL 17 with pgRDF installed from upstream release artifacts",
		BundleDir:   "bundles/bundle-pg17-pgrdf",
		Image: ImageSpec{
			Registry:   "ghcr.io/sporaxis-com/ociger-pg17-pgrdf",
			PGMajor:    17,
			BaseImage:  "postgres:17-bookworm",
			FinalImage: "gcr.io/distroless/base-debian12:latest",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Local: LocalSpec{
			Prefix:    "ociger-",
			DataDir:   ".artifacts/ociger-pg17-pgrdf-smoke/pgdata",
			Network:   "ociger-pg17-pgrdf-net",
			Container: "ociger-pg17-pgrdf-smoke",
		},
	}

	df, bake, err := Render(spec)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	if !strings.Contains(df, "FROM alpine:3.20 AS pgrdf_fetch") {
		t.Fatalf("Dockerfile missing pgrdf fetch stage:\n%s", df)
	}

	if !strings.Contains(df, "https://github.com/styk-tv/pgRDF/releases/download/v0.5.1/pgrdf-0.5.1-pg17-glibc-${TARGETARCH}.tar.gz") {
		t.Fatalf("Dockerfile missing pinned pgRDF release asset URL:\n%s", df)
	}

	if !strings.Contains(df, "COPY --from=pgrdf_fetch /out/lib/pgrdf.so /out/usr/lib/postgresql/17/lib/pgrdf.so") {
		t.Fatalf("Dockerfile missing pgrdf shared library copy:\n%s", df)
	}

	if !strings.Contains(df, "COPY --from=pgrdf_fetch /out/share/extension/ /out/usr/share/postgresql/17/extension/") {
		t.Fatalf("Dockerfile missing pgrdf extension directory copy:\n%s", df)
	}

	if !strings.Contains(bake, `target "bundle-pg17-pgrdf"`) {
		t.Fatalf("Bake output missing pgrdf target name:\n%s", bake)
	}

	if !strings.Contains(bake, `dockerfile = "bundles/bundle-pg17-pgrdf/Dockerfile"`) {
		t.Fatalf("Bake output missing pgrdf Dockerfile path:\n%s", bake)
	}
}

func TestRenderPGRDFPGCKBundle(t *testing.T) {
	spec := Spec{
		Name:        "bundle-pg17-pgrdf-pgck",
		Description: "PostgreSQL 17 with pgRDF and pgCK installed from upstream published artifacts",
		BundleDir:   "bundles/bundle-pg17-pgrdf-pgck",
		Image: ImageSpec{
			Registry:   "ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck",
			PGMajor:    17,
			BaseImage:  "postgres:17-bookworm",
			FinalImage: "gcr.io/distroless/base-debian12:latest",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Local: LocalSpec{
			Prefix:    "ociger-",
			DataDir:   ".artifacts/ociger-pg17-pgrdf-pgck-smoke/pgdata",
			Network:   "ociger-pg17-pgrdf-pgck-net",
			Container: "ociger-pg17-pgrdf-pgck-smoke",
		},
	}

	df, bake, err := Render(spec)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	if !strings.Contains(df, "FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch") {
		t.Fatalf("Dockerfile missing pgck fetch stage:\n%s", df)
	}

	if !strings.Contains(df, `/bin/oras pull --output /work "ghcr.io/styk-tv/pgck:0.1.2-pg17-${TARGETARCH}"`) {
		t.Fatalf("Dockerfile missing pgck oras pull:\n%s", df)
	}

	if !strings.Contains(df, "COPY --from=pgck_fetch /work/lib/pgck.so /out/usr/lib/postgresql/17/lib/pgck.so") {
		t.Fatalf("Dockerfile missing pgck shared library copy:\n%s", df)
	}

	if !strings.Contains(df, "COPY --from=pgck_fetch /work/share/extension/ /out/usr/share/postgresql/17/extension/") {
		t.Fatalf("Dockerfile missing pgck extension directory copy:\n%s", df)
	}

	if !strings.Contains(df, "ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgck") {
		t.Fatalf("Dockerfile missing pgck preload environment:\n%s", df)
	}

	if !strings.Contains(bake, `target "bundle-pg17-pgrdf-pgck"`) {
		t.Fatalf("Bake output missing pgck target name:\n%s", bake)
	}

	if !strings.Contains(bake, `dockerfile = "bundles/bundle-pg17-pgrdf-pgck/Dockerfile"`) {
		t.Fatalf("Bake output missing pgck Dockerfile path:\n%s", bake)
	}
}

func TestRenderCorePG17MicroBundle(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17-micro",
		Description: "Minimal embedded PostgreSQL 17 micro runtime",
		BundleDir:   "bundles/core-pg17-micro",
		Image: ImageSpec{
			Registry:       "ghcr.io/sporaxis-com/ociger-core-pg17-micro",
			PGMajor:        17,
			BaseImage:      "postgres:17-bookworm",
			FinalImage:     "scratch",
			RuntimeProfile: "micro",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Ports: []PortSpec{
			{Name: "postgres", ContainerPort: 5432},
		},
		Local: LocalSpec{
			Prefix:    "ociger-",
			DataDir:   ".artifacts/ociger-core-pg17-micro-smoke/pgdata",
			Network:   "ociger-core-pg17-micro-net",
			Container: "ociger-core-pg17-micro-smoke",
		},
	}

	df, _, err := Render(spec)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	assertMicroRuntimeContract(t, df)
}

func TestRenderCorePG17NATSBundle(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17-nats",
		Description: "Minimal embedded PostgreSQL 17 runtime with NATS",
		BundleDir:   "bundles/core-pg17-nats",
		Image: ImageSpec{
			Registry:   "ghcr.io/sporaxis-com/ociger-core-pg17-nats",
			PGMajor:    17,
			BaseImage:  "postgres:17-bookworm",
			FinalImage: "gcr.io/distroless/base-debian12:latest",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Ports: []PortSpec{
			{Name: "postgres", ContainerPort: 5432},
			{Name: "nats", ContainerPort: 4222},
			{Name: "nats-websocket", ContainerPort: 9222},
		},
		Services: ServiceSpec{
			NATS: &NATSServiceSpec{
				SourceImage:   "nats:2.14.1-scratch",
				CorePort:      4222,
				WebSocketPort: 9222,
				JetStream:     false,
			},
		},
		Local: LocalSpec{
			Prefix:    "ociger-",
			DataDir:   ".artifacts/ociger-core-pg17-nats-smoke/pgdata",
			Network:   "ociger-core-pg17-nats-net",
			Container: "ociger-core-pg17-nats-smoke",
		},
	}

	df, _, err := Render(spec)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	for _, want := range []string{
		"FROM nats:2.14.1-scratch AS nats_source",
		"COPY --from=nats_source /nats-server /out/usr/local/bin/nats-server",
		"EXPOSE 5432 4222 9222",
		"COPY bundles/core-pg17-nats/nats-server.conf /etc/nats/nats-server.conf",
		`ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]`,
	} {
		if !strings.Contains(df, want) {
			t.Fatalf("Dockerfile missing %q:\n%s", want, df)
		}
	}
}

func TestRenderCorePG17NATSMicroBundle(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17-nats-micro",
		Description: "Minimal embedded PostgreSQL 17 micro runtime with NATS",
		BundleDir:   "bundles/core-pg17-nats-micro",
		Image: ImageSpec{
			Registry:       "ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro",
			PGMajor:        17,
			BaseImage:      "postgres:17-bookworm",
			FinalImage:     "scratch",
			RuntimeProfile: "micro",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Ports: []PortSpec{
			{Name: "postgres", ContainerPort: 5432},
			{Name: "nats", ContainerPort: 4222},
			{Name: "nats-websocket", ContainerPort: 9222},
		},
		Services: ServiceSpec{
			NATS: &NATSServiceSpec{
				SourceImage:   "nats:2.14.1-scratch",
				CorePort:      4222,
				WebSocketPort: 9222,
				JetStream:     false,
			},
		},
		Local: LocalSpec{
			Prefix:    "ociger-",
			DataDir:   ".artifacts/ociger-core-pg17-nats-micro-smoke/pgdata",
			Network:   "ociger-core-pg17-nats-micro-net",
			Container: "ociger-core-pg17-nats-micro-smoke",
		},
	}

	df, _, err := Render(spec)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	for _, want := range []string{
		"FROM nats:2.14.1-scratch AS nats_source",
		"COPY --from=nats_source /nats-server /out/usr/local/bin/nats-server",
		"EXPOSE 5432 4222 9222",
		"COPY bundles/core-pg17-nats-micro/nats-server.conf /etc/nats/nats-server.conf",
		`ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]`,
	} {
		if !strings.Contains(df, want) {
			t.Fatalf("Dockerfile missing %q:\n%s", want, df)
		}
	}

	assertMicroRuntimeContract(t, df)
}

func TestRenderRejectsNATSPortMismatch(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17-nats",
		Description: "Minimal embedded PostgreSQL 17 runtime with NATS",
		BundleDir:   "bundles/core-pg17-nats",
		Image: ImageSpec{
			Registry:   "ghcr.io/sporaxis-com/ociger-core-pg17-nats",
			PGMajor:    17,
			BaseImage:  "postgres:17-bookworm",
			FinalImage: "gcr.io/distroless/base-debian12:latest",
		},
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Ports: []PortSpec{
			{Name: "postgres", ContainerPort: 5432},
			{Name: "nats", ContainerPort: 4222},
		},
		Services: ServiceSpec{
			NATS: &NATSServiceSpec{
				SourceImage:   "nats:2.14.1-scratch",
				CorePort:      4222,
				WebSocketPort: 9222,
				JetStream:     false,
			},
		},
	}

	_, _, err := Render(spec)
	if err == nil {
		t.Fatal("Render returned nil error for mismatched NATS ports")
	}
	if !strings.Contains(err.Error(), "nats") || !strings.Contains(err.Error(), "9222") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderAbsoluteBundlePathKeepsNATSCopySourceRelative(t *testing.T) {
	repoRoot := testRepoRoot(t)
	bundlePath, bundleDir := writeTestBundle(t, repoRoot, "absolute-render-", []byte(`
name: core-pg17-nats-absolute-render
description: PostgreSQL 17 runtime with NATS rendered from an absolute bundle path
image:
  registry: ghcr.io/sporaxis-com/ociger-core-pg17-nats-absolute-render
  pg_major: 17
  base_image: postgres:17-bookworm
  final_image: scratch
platforms:
  - linux/amd64
ports:
  - name: postgres
    container_port: 5432
  - name: nats
    container_port: 4222
  - name: nats-websocket
    container_port: 9222
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
    jetstream: false
local:
  prefix: ociger-
  data_dir: .artifacts/ociger-core-pg17-nats-absolute-render/pgdata
  network: ociger-core-pg17-nats-absolute-render-net
  container: ociger-core-pg17-nats-absolute-render
`))

	withWorkingDir(t, repoRoot, func() {
		spec, err := Load(bundlePath)
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}

		df, _, err := Render(spec)
		if err != nil {
			t.Fatalf("Render returned error: %v", err)
		}

		wantCopy := "COPY " + filepath.ToSlash(filepath.Join("bundles", filepath.Base(bundleDir), "nats-server.conf")) + " /etc/nats/nats-server.conf"
		if !strings.Contains(df, wantCopy) {
			t.Fatalf("Dockerfile missing %q:\n%s", wantCopy, df)
		}
		if strings.Contains(df, filepath.ToSlash(bundleDir)) {
			t.Fatalf("Dockerfile should not embed absolute bundle path %q:\n%s", bundleDir, df)
		}
	})
}
