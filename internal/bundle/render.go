package bundle

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	natsconfig "github.com/sporaxis-com/oci-germination/internal/nats"
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

{{- if hasNATS . }}
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS supervisor_build
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd/ociger-supervisor/main.go ./cmd/ociger-supervisor/main.go
COPY internal/supervisor ./internal/supervisor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/ociger-supervisor ./cmd/ociger-supervisor

{{ end }}
{{- if includesPGRDF . }}
FROM alpine:3.20 AS pgrdf_fetch
ARG TARGETARCH
RUN apk add --no-cache curl tar
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  curl -fsSL -o /tmp/pgrdf.tar.gz "https://github.com/styk-tv/pgRDF/releases/download/v{{ .Extensions.PGRDF.Version }}/pgrdf-{{ .Extensions.PGRDF.Version }}-pg{{ .Image.PGMajor }}-glibc-${TARGETARCH}.tar.gz"; \
  mkdir -p /out; \
  tar -xzf /tmp/pgrdf.tar.gz -C /out --strip-components=1; \
  test -s /out/lib/pgrdf.so; \
  test -s /out/share/extension/pgrdf.control; \
  test -s /out/share/extension/pgrdf--{{ .Extensions.PGRDF.Version }}.sql
{{ end }}
{{- if includesPGCK . }}

FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgck_fetch
ARG TARGETARCH
WORKDIR /work
RUN set -eux; \
  case "$TARGETARCH" in amd64|arm64) ;; *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac; \
  /bin/oras pull --output /work "ghcr.io/styk-tv/pgck:{{ .Extensions.PGCK.Version }}-pg{{ .Image.PGMajor }}-${TARGETARCH}"; \
  test -s /work/lib/pgck.so; \
  test -s /work/share/extension/pgck.control; \
  test -s /work/share/extension/pgck--{{ .Extensions.PGCK.Version }}.sql
{{ end }}
{{- if hasNATS . }}

FROM {{ .Services.NATS.SourceImage }} AS nats_source
{{ end }}

FROM {{ .Image.BaseImage }} AS postgres_source
{{- if isMicro . }}
RUN set -eux; \
  mkdir -p /out/bin /out/etc /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib /out/usr/share/postgresql /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension /out/usr/share/postgresql/{{ .Image.PGMajor }}/tsearch_data /out/usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets /out/var/lib/postgresql /out/var/run/postgresql; \
  cp -L /bin/sh /out/bin/sh; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb /out/usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/dict_snowball.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/dict_snowball.so; \
  cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/catalog_version /out/usr/share/postgresql/{{ .Image.PGMajor }}/catalog_version; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/errcodes.txt /out/usr/share/postgresql/{{ .Image.PGMajor }}/errcodes.txt; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/postgres.bki /out/usr/share/postgresql/{{ .Image.PGMajor }}/postgres.bki; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/information_schema.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/information_schema.sql; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/snowball_create.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/snowball_create.sql; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/sql_features.txt /out/usr/share/postgresql/{{ .Image.PGMajor }}/sql_features.txt; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/system_functions.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/system_functions.sql; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/system_views.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/system_views.sql; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/system_constraints.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/system_constraints.sql; \
  cp -a /usr/share/postgresql/{{ .Image.PGMajor }}/tsearch_data/. /out/usr/share/postgresql/{{ .Image.PGMajor }}/tsearch_data/; \
  cp -a /usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets/. /out/usr/share/postgresql/{{ .Image.PGMajor }}/timezonesets/; \
  cp -L /usr/share/postgresql/postgresql.conf.sample /out/usr/share/postgresql/postgresql.conf.sample; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/postgresql.conf.sample /out/usr/share/postgresql/{{ .Image.PGMajor }}/postgresql.conf.sample; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/pg_hba.conf.sample /out/usr/share/postgresql/{{ .Image.PGMajor }}/pg_hba.conf.sample; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/pg_ident.conf.sample /out/usr/share/postgresql/{{ .Image.PGMajor }}/pg_ident.conf.sample; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/pg_service.conf.sample /out/usr/share/postgresql/{{ .Image.PGMajor }}/pg_service.conf.sample; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/psqlrc.sample /out/usr/share/postgresql/{{ .Image.PGMajor }}/psqlrc.sample; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/extension/plpgsql.control /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/plpgsql.control; \
  cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/extension/plpgsql--*.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/; \
  printf 'root:x:0:0:root:/root:/bin/sh\npostgres:x:999:999:postgres:/var/lib/postgresql:/bin/sh\n' > /out/etc/passwd; \
  printf 'root:x:0:\npostgres:x:999:\n' > /out/etc/group; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/postgres | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/bin/initdb | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/plpgsql.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  ldd /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/dict_snowball.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out
{{- else }}
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
{{- end }}
{{- if includesPGRDF . }}
COPY --from=pgrdf_fetch /out/lib/pgrdf.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgrdf.so
COPY --from=pgrdf_fetch /out/share/extension/ /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/
{{ end }}
{{- if includesPGCK . }}
COPY --from=pgck_fetch /work/lib/pgck.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgck.so
COPY --from=pgck_fetch /work/share/extension/ /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/
{{ end }}
{{- if microHasExtensionDeps . }}
RUN set -eux; \
  if [ -f /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgrdf.so ]; then \
    ldd /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgrdf.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  fi; \
  if [ -f /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgck.so ]; then \
    cp -L /usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgcrypto.so /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgcrypto.so; \
    cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/extension/pgcrypto.control /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/pgcrypto.control; \
    cp -L /usr/share/postgresql/{{ .Image.PGMajor }}/extension/pgcrypto--*.sql /out/usr/share/postgresql/{{ .Image.PGMajor }}/extension/; \
    ldd /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgck.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
    ldd /out/usr/lib/postgresql/{{ .Image.PGMajor }}/lib/pgcrypto.so | tr -s '[:space:]' '\n' | grep '^/' | sort -u | xargs -r -I '{}' cp --parents '{}' /out; \
  fi
{{ end }}

FROM {{ .Image.FinalImage }}
ENV PGDATA=/var/lib/postgresql/data
{{- if needsPreload . }}
ENV OCIGER_SHARED_PRELOAD_LIBRARIES={{ preloadLibs . }}
{{ end }}
COPY --from=postgres_source /out/ /
COPY --from=launcher_build /out/ociger-pg-launcher /usr/local/bin/ociger-pg-launcher
{{- if hasNATS . }}
COPY --from=supervisor_build /out/ociger-supervisor /usr/local/bin/ociger-supervisor
COPY --from=nats_source /nats-server /usr/local/bin/nats-server
COPY {{ .BundleDir }}/nats-server.conf /etc/nats/nats-server.conf
EXPOSE {{ exposedPorts . }}
{{ end }}
VOLUME ["/var/lib/postgresql/data"]
{{- if hasNATS . }}
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
{{- else }}
ENTRYPOINT ["/usr/local/bin/ociger-pg-launcher"]
{{- end }}
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

type renderedAssets struct {
	dockerfile string
	bake       string
	natsConfig string
}

func Render(spec Spec) (string, string, error) {
	assets, err := renderAssets(spec)
	if err != nil {
		return "", "", err
	}

	return assets.dockerfile, assets.bake, nil
}

func Write(spec Spec, dockerfilePath string, bakePath string) error {
	assets, err := renderAssets(spec)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dockerfilePath), 0o755); err != nil {
		return fmt.Errorf("create dockerfile dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(bakePath), 0o755); err != nil {
		return fmt.Errorf("create bake dir: %w", err)
	}

	if err := os.WriteFile(dockerfilePath, []byte(assets.dockerfile), 0o644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}
	if err := os.WriteFile(bakePath, []byte(assets.bake), 0o644); err != nil {
		return fmt.Errorf("write bake file: %w", err)
	}
	if assets.natsConfig != "" {
		confPath := filepath.Join(filepath.Dir(dockerfilePath), "nats-server.conf")
		if err := os.WriteFile(confPath, []byte(assets.natsConfig), 0o644); err != nil {
			return fmt.Errorf("write nats config: %w", err)
		}
	}

	return nil
}

func renderAssets(spec Spec) (renderedAssets, error) {
	spec, err := normalizeSpec(spec)
	if err != nil {
		return renderedAssets{}, err
	}

	df, err := executeTemplate(dockerfileTemplate, spec)
	if err != nil {
		return renderedAssets{}, err
	}

	bake, err := executeTemplate(bakeTemplate, spec)
	if err != nil {
		return renderedAssets{}, err
	}

	assets := renderedAssets{
		dockerfile: df,
		bake:       bake,
	}
	if spec.Services.NATS != nil {
		assets.natsConfig = natsconfig.Render(natsconfig.Config{
			CorePort:      spec.Services.NATS.CorePort,
			WebSocketPort: spec.Services.NATS.WebSocketPort,
			JetStream:     spec.Services.NATS.JetStream,
		})
	}

	return assets, nil
}

func normalizeSpec(spec Spec) (Spec, error) {
	profile := strings.ToLower(strings.TrimSpace(spec.Image.RuntimeProfile))
	if profile == "" {
		profile = "stable"
	}
	if profile != "stable" && profile != "micro" {
		return spec, fmt.Errorf("invalid runtime profile %q: must be stable or micro", spec.Image.RuntimeProfile)
	}
	spec.Image.RuntimeProfile = profile

	if spec.Services.NATS != nil {
		portSet := make(map[int]struct{}, len(spec.Ports))
		for _, port := range spec.Ports {
			portSet[port.ContainerPort] = struct{}{}
		}

		var missing []string
		if _, ok := portSet[spec.Services.NATS.CorePort]; !ok {
			missing = append(missing, fmt.Sprintf("core_port %d", spec.Services.NATS.CorePort))
		}
		if _, ok := portSet[spec.Services.NATS.WebSocketPort]; !ok {
			missing = append(missing, fmt.Sprintf("websocket_port %d", spec.Services.NATS.WebSocketPort))
		}
		if len(missing) > 0 {
			return spec, fmt.Errorf("nats service ports must be present in ports metadata: missing %s", strings.Join(missing, ", "))
		}
	}

	if spec.Extensions.PGRDF != nil && strings.TrimSpace(spec.Extensions.PGRDF.Version) == "" {
		return spec, fmt.Errorf("pgrdf version must be set when pgrdf extension is enabled")
	}
	if spec.Extensions.PGCK != nil && strings.TrimSpace(spec.Extensions.PGCK.Version) == "" {
		return spec, fmt.Errorf("pgck version must be set when pgck extension is enabled")
	}

	return spec, nil
}

func executeTemplate(source string, spec Spec) (string, error) {
	tpl, err := template.New("tpl").Funcs(template.FuncMap{
		"exposedPorts": func(spec Spec) string {
			ports := make([]string, 0, len(spec.Ports))
			for _, port := range spec.Ports {
				ports = append(ports, fmt.Sprintf("%d", port.ContainerPort))
			}

			return strings.Join(ports, " ")
		},
		"hasNATS": func(spec Spec) bool {
			return spec.Services.NATS != nil
		},
		"includesPGRDF": func(spec Spec) bool {
			return spec.Extensions.PGRDF != nil
		},
		"includesPGCK": func(spec Spec) bool {
			return spec.Extensions.PGCK != nil
		},
		"needsPreload": func(spec Spec) bool {
			return spec.Extensions.PGRDF != nil || spec.Extensions.PGCK != nil
		},
		"preloadLibs": func(spec Spec) string {
			var libs []string
			if spec.Extensions.PGRDF != nil {
				libs = append(libs, "pgrdf")
			}
			if spec.Extensions.PGCK != nil {
				libs = append(libs, "pgck")
			}
			return strings.Join(libs, ",")
		},
		"isMicro": func(spec Spec) bool {
			return spec.Image.RuntimeProfile == "micro"
		},
		"microHasExtensionDeps": func(spec Spec) bool {
			return spec.Image.RuntimeProfile == "micro" && (spec.Extensions.PGRDF != nil || spec.Extensions.PGCK != nil)
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
