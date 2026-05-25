package bundle

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const dockerfileTemplate = `# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS launcher_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-pg-launcher/main.go ./cmd/ociger-pg-launcher/main.go
COPY internal/launcher ./internal/launcher
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-pg-launcher ./cmd/ociger-pg-launcher

FROM {{ .Image.BaseImage }} AS postgres_source
RUN set -eux; \
  mkdir -p /out/bin /out/usr/lib/postgresql /out/usr/share/postgresql /out/etc /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /bin/sh /out/bin/sh; \
  cp -a /usr/lib/postgresql/{{ .Image.PGMajor }} /out/usr/lib/postgresql/; \
  cp -a /usr/share/postgresql/{{ .Image.PGMajor }} /out/usr/share/postgresql/; \
  cp -L /usr/share/postgresql/postgresql.conf.sample /out/usr/share/postgresql/postgresql.conf.sample; \
  cp /etc/passwd /out/etc/passwd; \
  cp /etc/group /out/etc/group; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres | tr ' ' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb | tr ' ' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out

FROM {{ .Image.FinalImage }}
ENV PGDATA=/var/lib/postgresql/data
COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]
`

const bakeTemplate = `group "default" {
  targets = ["core-pg17"]
}

target "core-pg17" {
  context = "."
  dockerfile = "bundles/core-pg17/Dockerfile"
  tags = ["{{ .Image.Registry }}:dev"]
  platforms = [{{ range $i, $p := .Platforms }}{{ if $i }}, {{ end }}"{{ $p }}"{{ end }}]
}
`

func Render(spec Spec) (string, string, error) {
	df, err := executeTemplate(dockerfileTemplate, spec)
	if err != nil {
		return "", "", err
	}

	bake, err := executeTemplate(bakeTemplate, spec)
	if err != nil {
		return "", "", err
	}

	return df, bake, nil
}

func Write(spec Spec, dockerfilePath string, bakePath string) error {
	df, bake, err := Render(spec)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dockerfilePath), 0o755); err != nil {
		return fmt.Errorf("create dockerfile dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(bakePath), 0o755); err != nil {
		return fmt.Errorf("create bake dir: %w", err)
	}

	if err := os.WriteFile(dockerfilePath, []byte(df), 0o644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}
	if err := os.WriteFile(bakePath, []byte(bake), 0o644); err != nil {
		return fmt.Errorf("write bake file: %w", err)
	}

	return nil
}

func executeTemplate(source string, spec Spec) (string, error) {
	tpl, err := template.New("tpl").Parse(source)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, spec); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()) + "\n", nil
}
