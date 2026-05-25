package bundle

import (
	"strings"
	"testing"
)

func TestRenderCoreBundle(t *testing.T) {
	spec := Spec{
		Name:        "core-pg17",
		Description: "Minimal embedded PostgreSQL 17 runtime",
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
}
