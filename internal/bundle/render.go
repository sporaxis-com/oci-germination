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

{{- if includesPGRDF . }}
FROM alpine:3.20 AS pgrdf_fetch
ARG TARGETARCH
RUN apk add --no-cache curl tar
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  curl -fsSL -o /tmp/pgrdf.tar.gz "https://github.com/styk-tv/pgRDF/releases/download/v0.5.1/pgrdf-0.5.1-pg{{ .Image.PGMajor }}-glibc-${TARGETARCH}.tar.gz"; \
  mkdir -p /out; \
  tar -xzf /tmp/pgrdf.tar.gz -C /out --strip-components=1; \
  test -s /out/lib/pgrdf.so; \
  test -s /out/share/extension/pgrdf.control; \
  test -s /out/share/extension/pgrdf--0.5.1.sql
{{ end }}
{{- if includesPGCK . }}

FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgck:0.1.2-pg{{ .Image.PGMajor }}-${TARGETARCH}"; \
  test -s /work/lib/pgck.so; \
  test -s /work/share/extension/pgck.control; \
  test -s /work/share/extension/pgck--0.1.2.sql
{{ end }}

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
{{- if includesPGRDF . }}
COPY --from=pgrdf_fetch /out/lib/pgrdf.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgrdf.so
COPY --from=pgrdf_fetch /out/share/extension/ /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/
{{ end }}
{{- if includesPGCK . }}
COPY --from=pgck_fetch /work/lib/pgck.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgck.so
COPY --from=pgck_fetch /work/share/extension/ /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/
{{ end }}

FROM {{ .Image.FinalImage }}
ENV PGDATA=/var/lib/postgresql/data
{{- if includesPGCK . }}
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgck
{{ end }}
COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]
`

const bakeTemplate = `group "default" {
  targets = ["{{ .Name }}"]
}

target "{{ .Name }}" {
  context = "."
  dockerfile = "{{ .BundleDir }}/Dockerfile"
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
	tpl, err := template.New("tpl").Funcs(template.FuncMap{
		"includesPGRDF": func(spec Spec) bool {
			return spec.Name == "bundle-pg17-pgrdf" || spec.Name == "bundle-pg17-pgrdf-pgck"
		},
		"includesPGCK": func(spec Spec) bool {
			return spec.Name == "bundle-pg17-pgrdf-pgck"
		},
	}).Parse(source)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, spec); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()) + "\n", nil
}
