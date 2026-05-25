package bundle

import (
	"strings"
	"testing"
)

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
