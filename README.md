# OCI Germination

`oci-germination` publishes small OCI PostgreSQL bundles without rebuilding upstream extension projects in this repo.

**CI/CD:** New bundle versions are published automatically via GitHub Actions when version tags are pushed (see [CONTRIBUTING.CI.md](CONTRIBUTING.CI.md) for tag convention and publishing workflow).

Current public releases:

**Infrastructure bundles:**
- `core-pg17` — minimal PostgreSQL 17 runtime
- `core-pg17-micro` — smaller scratch-based PostgreSQL 17 runtime
- `core-pg17-nats` — PostgreSQL 17 + embedded `nats-server` + WebSocket listener
- `core-pg17-nats-micro` — smaller PostgreSQL 17 + embedded `nats-server` + WebSocket listener
- `pg17-pgrdf` — PostgreSQL 17 + `pgRDF 0.5.1`
- `pg17-pgrdf-pgck` — PostgreSQL 17 + `pgRDF 0.5.1` + `pgCK 0.1.2`
- `pg17-pgrdf-pgck-nats` — PostgreSQL 17 + `pgRDF 0.5.1` + `pgCK 0.1.2` + `nats-server 2.14.1`
- `pg17-pgrdf-pgck-nats-micro` — smaller PostgreSQL 17 + `pgRDF 0.5.1` + `pgCK 0.1.2` + `nats-server 2.14.1`

**Web-serving bundles (CKP v3.8):**
- `pg17-pgrdf-pgck-web-cklib` — PostgreSQL 17 + pgRDF + pgCK + FastAPI web UI + cklib (no NATS; standard variant)
- `ck-allinone` (v3.8-rc2) — **[CKP Development Default]** All-in-one stack for Concept Kernel: PostgreSQL 17 + pgRDF + pgCK + pgckweb + cklib + NATS core + WSS bridge + supervisor orchestration

Next target:

- `pg17-pgrdf-pgck` ontology smoke — load vendored `3.8` fixtures and run simple validate/query checks

## Published Images

All published images are multi-arch manifest lists for `linux/amd64` and `linux/arm64`.

| Bundle | OCI image | Platforms | Size | Verified behavior |
| --- | --- | --- | --- | --- |
| `core-pg17` | `ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2` | `amd64`, `arm64` | `139.5 / 53.1 MiB` | boots, initializes `PGDATA`, creates DB/table/row, proves relation file on host bind mount |
| `core-pg17-micro` | `ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.1` | `amd64`, `arm64` | `73.8 / 29.2 MiB` | same SQL/data proof as `core-pg17`, with a smaller scratch final image |
| `core-pg17-nats` | `ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.1` | `amd64`, `arm64` | `157.9 / 60.3 MiB` | `core-pg17` proof plus NATS core on `4222` and WebSocket on `9222` |
| `core-pg17-nats-micro` | `ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.1` | `amd64`, `arm64` | `92.2 / 36.4 MiB` | `core-pg17-micro` proof plus NATS core on `4222` and WebSocket on `9222` |
| `pg17-pgrdf` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.1` | `amd64`, `arm64` | `not measured yet` | `CREATE EXTENSION pgrdf`; `pg_available_extensions.default_version=0.5.1`; `pg_extension.extversion=0.5.1`; `pgrdf.version()=0.5.1` |
| `pg17-pgrdf-pgck` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1` | `amd64`, `arm64` | `151.7 / 57.8 MiB` | `pgck` preloaded by default; `CREATE EXTENSION pgrdf`; `CREATE EXTENSION pgck CASCADE`; `pgrdf.version()=0.5.1`; `pgck_version()=pgck 0.1.2 (rc3)` |
| `pg17-pgrdf-pgck-nats` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:v0.1.1` | `amd64`, `arm64` | `170.1 / 65.1 MiB` | triple-bundle proof plus NATS core on `4222`, WebSocket on `9222`, and one-image host relation-file proof |
| `pg17-pgrdf-pgck-nats-micro` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.1` | `amd64`, `arm64` | `104.5 / 41.3 MiB` | all-in-one proof on the micro runtime line: `pgrdf`, `pgck`, NATS, and host relation-file proof |
| `pg17-pgrdf-pgck-web-cklib` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:1.0.0` | `amd64`, `arm64` | `~200 / TBD` | ✓ Published | **Standard variant** — PostgreSQL 17 + pgRDF 0.5.1 + pgCK 0.1.2 + pgckweb 0.1.0 + cklib 1.2.0 (no NATS) |
| `ck-allinone` | `ghcr.io/sporaxis-com/ociger-ck-allinone:v3.8-rc2` | `amd64`, `arm64` | `~150 / TBD` | ✓ Published | **CKP Development Default** — All-in-one for Concept Kernel: pgckweb + cklib + NATS core (4222) + WSS (9222) + supervisor. Extends `ociger-pg17-pgrdf-pgck-nats-micro:v0.1.1` base. |

Size values are `uncompressed / compressed` local measurements on `linux/arm64`. New bundles now published to GHCR with multi-platform manifests.

Pinned manifest digests:

- `core-pg17-v0.1.2` → `sha256:bd11e0bc6b39577e1e8e946dc28fcf3a7cea24524472a97c68c408a80ef4e3ac`
- `core-pg17-micro:v0.1.1` → `sha256:730abc0ca4104fbb9da37228ca9e941bd59d03e997ca5ae73ac06636ad0ba92c`
- `core-pg17-nats:v0.1.1` → `sha256:8732386137c50770ca67ca74d8066ec7948b7294c1c1bdb40d055be88f6481ea`
- `core-pg17-nats-micro:v0.1.1` → `sha256:d5b45d0abe70f7df63d159e331c2baefc6a78dd2098e8907cb5251070640e552`
- `pg17-pgrdf:v0.1.1` → `sha256:31e2bdbb34d2ddf3fec609eb748ef9bec0707a019adb386d0d095463abb20d61`
- `pg17-pgrdf-pgck:v0.1.1` → `sha256:16312830aed761a6201c111e31ff949d5179de613dfdf39d060e8a3c906c59c2`
- `pg17-pgrdf-pgck-nats:v0.1.1` → `sha256:8a7e8c42b3557a1b7958006ad42bf53423bd75512a9c3db530dbe0c6ae4f84bf`
- `pg17-pgrdf-pgck-nats-micro:v0.1.1` → `sha256:aef55d2a43c94072564fc0934c25d986cc37402788422ef054831536e8262e42`

## Bundle Chain

| Bundle | Extends | Adds | Output | Notes |
| --- | --- | --- | --- | --- |
| `core-pg17` | `postgres:17-bookworm` runtime files + distroless final image | none | `ociger-core-pg17-min` | stable minimal PostgreSQL 17 runtime |
| `core-pg17-micro` | selective PostgreSQL runtime files + `scratch` final image | none | `ociger-core-pg17-micro` | smaller runtime family with the same SQL/data contract |
| `core-pg17-nats` | `core-pg17` runtime shape | `nats-server 2.14.1`, tiny supervisor, WebSocket listener | `ociger-core-pg17-nats` | JetStream off by default |
| `core-pg17-nats-micro` | `core-pg17-micro` runtime shape | `nats-server 2.14.1`, tiny supervisor, WebSocket listener | `ociger-core-pg17-nats-micro` | smaller service bundle, JetStream off by default |
| `pg17-pgrdf` | `core-pg17` runtime shape | upstream `pgRDF 0.5.1` artifact | `ociger-pg17-pgrdf` | working extension bundle |
| `pg17-pgrdf-pgck` | `core-pg17` runtime shape | upstream `pgRDF 0.5.1` + `pgCK 0.1.2` artifacts | `ociger-pg17-pgrdf-pgck` | triple bundle with `pgck` preloaded by default |
| `pg17-pgrdf-pgck-nats` | `core-pg17` runtime shape | upstream `pgRDF 0.5.1` + `pgCK 0.1.2` artifacts + `nats-server 2.14.1` | `ociger-pg17-pgrdf-pgck-nats` | all-in-one stable bundle with NATS and both extensions |
| `pg17-pgrdf-pgck-nats-micro` | `core-pg17-micro` runtime shape | upstream `pgRDF 0.5.1` + `pgCK 0.1.2` artifacts + `nats-server 2.14.1` | `ociger-pg17-pgrdf-pgck-nats-micro` | smaller all-in-one bundle with the same SQL/NATS contract |
| `pg17-pgrdf-pgck-web-cklib` | `pg17-pgrdf-pgck` (no NATS) | FastAPI web server (pgckweb 0.1.0) + cklib (CK.Lib.Js 1.2.0) OCI layer source | `ociger-pg17-pgrdf-pgck-web-cklib` | **Standard variant** — clean web UI + cklib, no messaging layer |
| `ck-allinone` (v3.8-rc2) | `pg17-pgrdf-pgck-nats-micro` (with NATS) | FastAPI pgckweb 0.1.0 + cklib 1.2.0 + inherited NATS infrastructure + ociger-supervisor for service orchestration | `ociger-ck-allinone` | **[CKP Development Default]** — Complete all-in-one stack for Concept Kernel development: all extensions + web UI + NATS (4222) + WSS bridge (9222) + orchestration |

The NATS service layer is reusable: it copies only `nats-server` from `nats:2.14.1-scratch`, renders a minimal config, and starts PostgreSQL plus NATS through the same tiny supervisor. `pg17-pgrdf-pgck-nats` and `pg17-pgrdf-pgck-nats-micro` prove the same pattern can carry both upstream extensions and the colocated message bus in one OCI image.

Web-serving bundles extend the PostgreSQL+extensions base with FastAPI and static file mounts. The cklib OCI layer is imported additively—no extraction—by referencing `ck-lib-js:1.2.0` in the builder stage and mounting files at `/cklib/` in the final image. For Concept Kernel development, use **ck-allinone** which adds NATS messaging infrastructure for agent coordination.

## One-Line Launch

Run the minimal core image:

```bash
docker run --rm -d \
  --name ociger-core-pg17 \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-core-pg17-data:/var/lib/postgresql/data" \
  -p 15432:5432 \
  ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2
```

Then connect:

```bash
psql -h 127.0.0.1 -p 15432 -U postgres -d postgres
```

Run the smaller `core-pg17-micro` image:

```bash
docker run --rm -d \
  --name ociger-core-pg17-micro \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-core-pg17-micro-data:/var/lib/postgresql/data" \
  -p 15435:5432 \
  ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.1
```

Run the `core-pg17-nats` image:

```bash
docker run --rm -d \
  --name ociger-core-pg17-nats \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-core-pg17-nats-data:/var/lib/postgresql/data" \
  -p 15436:5432 \
  -p 14222:4222 \
  -p 19222:9222 \
  ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.1
```

Run the smaller `core-pg17-nats-micro` image:

```bash
docker run --rm -d \
  --name ociger-core-pg17-nats-micro \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-core-pg17-nats-micro-data:/var/lib/postgresql/data" \
  -p 15437:5432 \
  -p 14223:4222 \
  -p 19223:9222 \
  ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.1
```

Then probe the NATS listeners:

```bash
nc 127.0.0.1 14222 < /dev/null
nc -zv 127.0.0.1 19222
```

Run the `pgRDF` bundle:

```bash
docker run --rm -d \
  --name ociger-pg17-pgrdf \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-pg17-pgrdf-data:/var/lib/postgresql/data" \
  -p 15433:5432 \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.1
```

Then verify the extension surface:

```bash
psql -h 127.0.0.1 -p 15433 -U postgres -d postgres \
  -c 'CREATE EXTENSION pgrdf; SELECT pgrdf.version();'
```

Run the triple bundle:

```bash
docker run --rm -d \
  --name ociger-pg17-pgrdf-pgck \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-pg17-pgrdf-pgck-data:/var/lib/postgresql/data" \
  -p 15434:5432 \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1
```

Then verify both extensions and their project-native version surfaces:

```bash
psql -h 127.0.0.1 -p 15434 -U postgres -d postgres \
  -c 'CREATE EXTENSION pgrdf; CREATE EXTENSION pgck CASCADE; SELECT pgrdf.version(), pgck_version();'
```

Run the all-in-one stable bundle:

```bash
docker run --rm -d \
  --name ociger-pg17-pgrdf-pgck-nats \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-pg17-pgrdf-pgck-nats-data:/var/lib/postgresql/data" \
  -p 15438:5432 \
  -p 34222:4222 \
  -p 39222:9222 \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:v0.1.1
```

Run the all-in-one micro bundle:

```bash
docker run --rm -d \
  --name ociger-pg17-pgrdf-pgck-nats-micro \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-pg17-pgrdf-pgck-nats-micro-data:/var/lib/postgresql/data" \
  -p 15439:5432 \
  -p 34223:4222 \
  -p 39223:9222 \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.1
```

Then verify extensions plus NATS:

```bash
psql -h 127.0.0.1 -p 15438 -U postgres -d postgres -c 'CREATE DATABASE ociger_demo;'
psql -h 127.0.0.1 -p 15438 -U postgres -d ociger_demo \
  -c 'CREATE EXTENSION pgrdf; CREATE EXTENSION pgck CASCADE; SELECT pgrdf.version(), pgck_version();'
nc 127.0.0.1 34222 < /dev/null
nc -zv 127.0.0.1 39222
```

### Standard variant (no NATS)

Run the `pg17-pgrdf-pgck-web-cklib` bundle (FastAPI + cklib, no messaging):

```bash
docker run --rm -d \
  --name ociger-pg17-pgrdf-pgck-web-cklib \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-pg17-pgrdf-pgck-web-cklib-data:/var/lib/postgresql/data" \
  -p 15440:5432 \
  -p 18000:8000 \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:1.0.0
```

Then verify FastAPI and cklib files:

```bash
curl http://127.0.0.1:18000/          # pgckweb root endpoint
curl http://127.0.0.1:18000/cklib/ck-client.js    # cklib JavaScript files
psql -h 127.0.0.1 -p 15440 -U postgres -d postgres \
  -c 'CREATE EXTENSION pgrdf; CREATE EXTENSION pgck CASCADE; SELECT pgrdf.version(), pgck_version();'
```

### CKP Development Default

Run the `ck-allinone` bundle (v3.8-rc2 — recommended for Concept Kernel development):

```bash
docker run --rm -d \
  --name ociger-ck-allinone \
  -e PGDATA=/var/lib/postgresql/data \
  -v "$PWD/ociger-ck-allinone-data:/var/lib/postgresql/data" \
  -p 15441:5432 \
  -p 18001:8000 \
  -p 14222:4222 \
  -p 19222:9222 \
  ghcr.io/sporaxis-com/ociger-ck-allinone:v3.8-rc2
```

Then verify the full stack (PostgreSQL + extensions + FastAPI + NATS + cklib):

```bash
curl http://127.0.0.1:18001/          # pgckweb with cklib
curl http://127.0.0.1:18001/cklib/ck-client.js    # cklib client library
psql -h 127.0.0.1 -p 15441 -U postgres -d postgres \
  -c 'CREATE EXTENSION pgrdf; CREATE EXTENSION pgck CASCADE; SELECT pgrdf.version(), pgck_version();'
nc 127.0.0.1 14222 < /dev/null   # NATS core (for clustering/messaging)
nc -zv 127.0.0.1 19222           # NATS WSS (browser client connectivity)
```

## Verification

Core proof:

```bash
bash scripts/smoke-core-pg17.sh
bash scripts/smoke-core-pg17.sh ghcr.io/sporaxis-com/ociger-core-pg17-min:core-pg17-v0.1.2
bash scripts/smoke-core-pg17-micro.sh
bash scripts/smoke-core-pg17-micro.sh ghcr.io/sporaxis-com/ociger-core-pg17-micro:v0.1.1
```

NATS bundle proof:

```bash
bash scripts/smoke-core-pg17-nats.sh
bash scripts/smoke-core-pg17-nats.sh ghcr.io/sporaxis-com/ociger-core-pg17-nats:v0.1.1
bash scripts/smoke-core-pg17-nats-micro.sh
bash scripts/smoke-core-pg17-nats-micro.sh ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:v0.1.1
```

`pgRDF` proof:

```bash
bash scripts/smoke-pg17-pgrdf.sh
bash scripts/smoke-pg17-pgrdf.sh ghcr.io/sporaxis-com/ociger-pg17-pgrdf:v0.1.1
```

Triple-bundle proof:

```bash
bash scripts/smoke-pg17-pgrdf-pgck.sh
bash scripts/smoke-pg17-pgrdf-pgck.sh ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1
```

All-in-one bundle proof:

```bash
bash scripts/smoke-pg17-pgrdf-pgck-nats.sh
bash scripts/smoke-pg17-pgrdf-pgck-nats.sh ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:v0.1.1
bash scripts/smoke-pg17-pgrdf-pgck-nats-micro.sh
bash scripts/smoke-pg17-pgrdf-pgck-nats-micro.sh ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.1
```

Standard variant proof (FastAPI + cklib, no NATS):

```bash
bash scripts/smoke-pg17-pgrdf-pgck-web-cklib.sh
bash scripts/smoke-pg17-pgrdf-pgck-web-cklib.sh ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:1.0.0
```

CKP Development Default proof (all-in-one with NATS):

```bash
bash scripts/smoke-ck-allinone.sh
bash scripts/smoke-ck-allinone.sh ghcr.io/sporaxis-com/ociger-ck-allinone:v3.8-rc2
```

Expected `pgRDF` smoke output:

```text
CREATE EXTENSION
pg_available_extensions.default_version=0.5.1
pg_extension.extversion=0.5.1
pgrdf.version()=0.5.1
```

Expected `core-pg17` and `core-pg17-micro` smoke output shape:

```text
CREATE DATABASE
CREATE TABLE
INSERT 0 1
pg_relation_filepath(public.demo_rows)=base/16384/16385
host_path=.../.artifacts/ociger-.../pgdata/base/16384/16385
relation_proof_method=host
```

Expected `core-pg17-nats` and `core-pg17-nats-micro` extra proof:

```text
ociger-core-pg17-nats... (x.x.x.x:9222) open
nats_info_line=INFO {... "version":"2.14.1" ...}
pg_relation_filepath(public.demo_rows)=base/16384/16385
relation_proof_method=host
```

Expected triple-bundle smoke output:

```text
CREATE EXTENSION
NOTICE:  installing required extension "pgcrypto"
CREATE EXTENSION
pgrdf.pg_available_extensions.default_version=0.5.1
pgck.pg_available_extensions.default_version=0.1.2
pgrdf.pg_extension.extversion=0.5.1
pgck.pg_extension.extversion=0.1.2
pgrdf.version()=0.5.1
pgck_version()=pgck 0.1.2 (rc3)
```

Expected all-in-one bundle smoke output shape:

```text
ociger-pg17-pgrdf-pgck-nats... (x.x.x.x:9222) open
CREATE DATABASE
CREATE EXTENSION
NOTICE:  installing required extension "pgcrypto"
CREATE EXTENSION
nats_info_line=INFO {... "version":"2.14.1" ...}
pgrdf.version()=0.5.1
pgck_version()=pgck 0.1.2 (rc3)
pg_relation_filepath(public.demo_rows)=base/...
relation_proof_method=host
```

## Local Development

Generate bundle outputs:

```bash
bash scripts/generate.sh
```

Build locally:

```bash
bash scripts/build-core-pg17.sh
bash scripts/build-core-pg17-micro.sh
bash scripts/build-core-pg17-nats.sh
bash scripts/build-core-pg17-nats-micro.sh
bash scripts/build-pg17-pgrdf.sh
bash scripts/build-pg17-pgrdf-pgck.sh
bash scripts/build-pg17-pgrdf-pgck-nats.sh
bash scripts/build-pg17-pgrdf-pgck-nats-micro.sh
```

If you need to force a local architecture:

```bash
OCI_GER_PLATFORM=linux/amd64 bash scripts/build-core-pg17-micro.sh
OCI_GER_PLATFORM=linux/amd64 bash scripts/build-core-pg17-nats-micro.sh
```

The same `OCI_GER_PLATFORM` override works for the other build scripts. Local smoke resources stay contained to `ociger-`-prefixed names and `.artifacts/ociger-*` paths.

## Repository Layout

- `bundles/core-pg17/` — core PostgreSQL bundle spec and generated build files
- `bundles/bundle-pg17-pgrdf/` — `pgRDF` bundle spec and generated build files
- `bundles/bundle-pg17-pgrdf-pgck/` — triple-bundle spec and generated build files
- `bundles/bundle-pg17-pgrdf-pgck-nats/` — stable all-in-one bundle with `pgRDF`, `pgCK`, and NATS
- `bundles/bundle-pg17-pgrdf-pgck-nats-micro/` — smaller all-in-one bundle with `pgRDF`, `pgCK`, and NATS
- `bundles/core-pg17-micro/` — smaller PostgreSQL 17 core bundle
- `bundles/core-pg17-nats/` — PostgreSQL 17 + NATS bundle
- `bundles/core-pg17-nats-micro/` — smaller PostgreSQL 17 + NATS bundle
- `cmd/ociger-gen/` — bundle generator
- `cmd/ociger-pg-launcher/` — minimal first-boot PostgreSQL launcher
- `cmd/ociger-supervisor/` — tiny multi-service supervisor for PostgreSQL + NATS
- `scripts/build-*.sh` — local native-arch image builds
- `scripts/smoke-*.sh` — contained local and public-image smoke tests
- `.github/workflows/` — release verification and GHCR publish workflows

## Release

Release tags:

- `core-pg17-vX.Y.Z`
- `core-pg17-micro-vX.Y.Z`
- `core-pg17-nats-vX.Y.Z`
- `core-pg17-nats-micro-vX.Y.Z`
- `pg17-pgrdf-vX.Y.Z`
- `pg17-pgrdf-pgck-vX.Y.Z`
- `pg17-pgrdf-pgck-nats-vX.Y.Z`
- `pg17-pgrdf-pgck-nats-micro-vX.Y.Z`

Current workflows:

- [core-pg17-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/core-pg17-release.yml)
- [core-pg17-micro-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/core-pg17-micro-release.yml)
- [core-pg17-nats-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/core-pg17-nats-release.yml)
- [core-pg17-nats-micro-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/core-pg17-nats-micro-release.yml)
- [pg17-pgrdf-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/pg17-pgrdf-release.yml)
- [pg17-pgrdf-pgck-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/pg17-pgrdf-pgck-release.yml)
- [pg17-pgrdf-pgck-nats-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/pg17-pgrdf-pgck-nats-release.yml)
- [pg17-pgrdf-pgck-nats-micro-release.yml](/Users/neoxr/git_sporaxis-com/oci-germination/.github/workflows/pg17-pgrdf-pgck-nats-micro-release.yml)

Release publish jobs only proceed when the tagged commit is on `origin/main` history.

Manual release example:

```bash
git push origin main
git tag core-pg17-vX.Y.Z
git tag core-pg17-micro-vX.Y.Z
git tag core-pg17-nats-vX.Y.Z
git tag core-pg17-nats-micro-vX.Y.Z
git tag pg17-pgrdf-vX.Y.Z
git tag pg17-pgrdf-pgck-vX.Y.Z
git tag pg17-pgrdf-pgck-nats-vX.Y.Z
git tag pg17-pgrdf-pgck-nats-micro-vX.Y.Z
git push origin \
  core-pg17-vX.Y.Z \
  core-pg17-micro-vX.Y.Z \
  core-pg17-nats-vX.Y.Z \
  core-pg17-nats-micro-vX.Y.Z \
  pg17-pgrdf-vX.Y.Z \
  pg17-pgrdf-pgck-vX.Y.Z \
  pg17-pgrdf-pgck-nats-vX.Y.Z \
  pg17-pgrdf-pgck-nats-micro-vX.Y.Z
```

## Defaults

Current images are optimized for local startup and smoke verification, not exposed production use:

- `listen_addresses='*'`
- `host all all all trust`
- NATS core listener on `4222`
- NATS WebSocket listener on `9222`
- JetStream disabled in the current NATS variants
- no password required for the default smoke path

Treat the current images as local/dev artifacts until stricter auth defaults are introduced.

## Upstream Inputs

- `pgRDF` repository: `https://github.com/styk-tv/pgRDF`
- `pgRDF` install docs: `https://pgrdf.styk.tv/v0.5/operations/install`
- `pgCK` repository: `https://github.com/styk-tv/pgCK`
- NATS runtime source image: `nats:2.14.1-scratch`
