---
title: "SPEC.OCI.BUNDLE.v0.3 — Authoritative Packaging Protocol for the Sporaxis-Com / ConceptKernel Fleet"
version: 0.3
date: 2026-05-29
status: Authoritative
supersedes: SPEC.OCI.BUNDLE.v0.2
audience: Every repo in the fleet that publishes OCI artifacts consumed by another fleet repo — currently oci-germination, pgRDF, pgCK (extension + pgck-web), CK.Lib.Js, and any future sibling.
---

# SPEC.OCI.BUNDLE.v0.3 — Authoritative Packaging Protocol

This specification supersedes v0.2 and becomes the binding contract for every OCI artifact published by any repo in the fleet. v0.2 stays valid for already-published artifacts (immutability); v0.3 governs every new publish from this spec's `date:` forward.

**What v0.3 settles, that v0.2 did not:**

- The recognized artifact **shapes** (filesystem-layer OCI vs tarball-in-OCI vs ORAS-only artifact) and which is preferred.
- The two consumption patterns — **additive composition** (preferred) and **extraction** (acceptable when upstream forces it) — and when each is permitted.
- The **per-artifact metadata** every release MUST publish (Pull URI, per-arch digests, aggregate digest, attestation, artifact type, designation label, …).
- The **attestation contract** binding `LATEST.md`, `PROVENANCE.md`, and downstream consumption.
- A **declarative bundle.yaml schema** that lets a generator (or a hand-author) describe both `layer_sources` (additive) and `extracted_sources` (extract) without ambiguity.
- The **upstream-notification duty** when a consumer is forced into extraction because the upstream ships a tarball-in-OCI shape — to coordinate migration to filesystem-layer OCI over time.

**Breaking changes vs v0.2:** None of the v0.2 fields are removed or renamed. `static_web:` continues to mean exactly what it meant in v0.2 (it is now a subset of `layer_sources:` with the routing semantics preserved). New fields are additive. v0.2 bundles continue to render under v0.3 generators with no schema change required.

---

## 1. Recognized Artifact Shapes

Every OCI artifact consumed under this spec MUST fall into exactly one of the three shapes below. Publishers SHOULD prefer shape A whenever the payload is a filesystem the consumer would want to merge in. Consumers MUST recognize all three.

### 1.1 Shape A — Filesystem-Layer OCI Image (preferred)

The artifact is an OCI image (single-platform) or OCI image index (multi-arch), whose tar layers expand directly into a filesystem usable via `FROM` + `COPY --from`. The image content at its root path is the payload, ready to be merged into a downstream image.

**Media type signature:**
```
application/vnd.oci.image.index.v1+json     (multi-arch index)
  → application/vnd.oci.image.manifest.v1+json   (per-platform)
    → application/vnd.oci.image.layer.v1.tar+gzip (filesystem tar)
```

**Example:**
```
ghcr.io/conceptkernel/ck-lib-js:1.3.11   # files at root: ck-client.js, ck-page.js, vendor/, …
ghcr.io/styk-tv/pgck-web:v0.2.4           # consolidated web/ dual-page Display/Board FastAPI runtime
                                          # image; full Python + uvicorn web.app:app at standard paths
```

`ck-lib-js:1.3.11` is the current spec-v0.3-aligned attested CK.Lib.Js release (it fills in the §2.1 Required + Recommended manifest labels; see the §8.1 worked example). `pgck-web:v0.2.4` is the current attested head — the consolidated-`web/` dual-page Display/Board runtime (post the `web_demo/`→`web/` consolidation; see §8.3) — it is developed into a full Shape A consumption example in §8.2. Per the pgCK pgck-web RESPONSE, v0.2.4 supersedes the earlier v0.2.3 recommendation.

**Consumer pattern (additive composition):**

```dockerfile
FROM ghcr.io/conceptkernel/ck-lib-js:1.3.11 AS cklib
FROM <base>
COPY --from=cklib / /app/cklib/
```

No extract step. The upstream is added to the consumer's filesystem by the OCI runtime itself.

### 1.2 Shape B — Tarball-in-OCI (acceptable, but signal for migration)

The artifact is a single-layer OCI artifact whose layer is itself a `.tar.gz` blob with a project-specific media type, intentionally narrow (e.g. only `lib/<ext>.so` + `share/extension/<ext>--<ver>.sql` + `<ext>.control`). The blob is opaque to OCI runtimes; the consumer must `oras pull` it and `tar -xz` the layer to see the files.

**Media type signature:**
```
application/vnd.oci.image.manifest.v1+json
  → application/vnd.<org>.<project>.<artifact-class>.v<N>+<format>
    e.g. application/vnd.styk.pgrdf.bundle.v1+tar
         application/vnd.styk.pgrdf.tarball.v1+gzip
```

**Example:**
```
ghcr.io/styk-tv/pgrdf-bundle:0.5.16-pg17-amd64   # single layer = pgrdf-0.5.16-pg17-glibc-amd64.tar.gz
ghcr.io/styk-tv/pgck:0.2.2-pg17-amd64             # same shape, pgCK extension
```

**Consumer pattern (extract — acceptable):**

```dockerfile
FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS pgrdf_fetch
ARG TARGETARCH
WORKDIR /work
RUN oras pull --output /work ghcr.io/styk-tv/pgrdf-bundle:0.5.16-pg17-${TARGETARCH} && \
    tar -xzf pgrdf-0.5.16-pg17-glibc-${TARGETARCH}.tar.gz --strip-components=1
# /work/lib/pgrdf.so and /work/share/extension/* now exist.
FROM <base>
COPY --from=pgrdf_fetch /work/lib/pgrdf.so /usr/lib/postgresql/17/lib/pgrdf.so
COPY --from=pgrdf_fetch /work/share/extension/ /usr/share/postgresql/17/extension/
```

**Signal-for-migration duty:** When a consumer is forced into the extract pattern, that consumer SHOULD send a NOTIFY (per SPEC.NOTIFIES.v0.3) to the upstream proposing a parallel Shape A artifact (e.g. `pgrdf-layer:0.5.16-pg17-<arch>` whose layer tar expands to `/usr/lib/postgresql/17/lib/pgrdf.so` + `/usr/share/postgresql/17/extension/...`). The migration is coordinated through NOTIFY threads, not forced.

Shape B remains valid indefinitely for projects whose distribution model genuinely fits a narrow leaf (e.g. `.so` plus a single SQL file). It is not deprecated; it is acknowledged as an acceptable shape with one downstream cost (extract step) and one upstream win (smaller, scoped leaves).

### 1.3 Shape C — ORAS Artifact (general — covers Shape B)

ORAS-pushable arbitrary OCI artifacts. Shape B is the most common instance. Other ORAS-pushed shapes (e.g. a `.wasm` blob, a single executable) follow the same handling: consume via `oras pull`, then process per the artifact type's documentation. Specify the artifact type via the manifest's `artifactType` field where supported.

---

## 2. Per-Artifact Metadata Contract (binding for every release)

Every artifact published under this spec MUST expose the following metadata, both in its OCI manifest (where structurally possible) and in the publishing repo's `LATEST.md`. This is what makes a release "complete" — agents and humans both need to be able to verify it without out-of-band knowledge.

### 2.1 OCI Manifest Labels (Shape A only)

For Shape A images, the manifest's config blob MUST set:

| Label | Required | Value example |
|---|---|---|
| `org.opencontainers.image.source` | Required | `https://github.com/ConceptKernel/CK.Lib.Js` |
| `org.opencontainers.image.version` | Required | `1.3.11` |
| `org.opencontainers.image.revision` | Required | git SHA of the source commit |
| `org.opencontainers.image.created` | Required | RFC 3339 UTC |
| `org.opencontainers.image.licenses` | Recommended | SPDX id (e.g. `MIT`) |
| `org.opencontainers.image.description` | Recommended | one-liner |
| `org.opencontainers.image.designation` | Optional | project-specific (e.g. `ckp:static` for CK.Lib.Js) |

CK.Lib.Js `1.3.11` populates all four Required labels plus `licenses=MIT` and `description` — it is the reference example of a fully-labelled Shape A leaf.

For `pgck-web` (the other Shape A image in the fleet), the expected Required labels are `image.source=https://github.com/styk-tv/pgCK`, `image.version` = the `pgck-web/vN.M.K` tag, `image.revision` = the source commit SHA, and `image.created` = the commit/build timestamp.

Shape B artifacts SHOULD set as many of these as possible via ORAS annotations.

### 2.2 `LATEST.md` Block Per Released Artifact

Every release that the publishing repo wants advertised MUST land a block in `LATEST.md` containing the following six fields, with this exact shape (humans and renderers both parse it):

```markdown
## <package> — `<version>`

<one-paragraph description>

| arch  | Pull URI                                | Also tagged | Digest                                                                  | Created (UTC)       |
|-------|-----------------------------------------|-------------|-------------------------------------------------------------------------|---------------------|
| amd64 | `<ref-amd64>`                           | `<aliases>` | `sha256:<per-platform-digest>`                                          | YYYY-MM-DD HH:MM:SS |
| arm64 | `<ref-arm64>`                           | `<aliases>` | `sha256:<per-platform-digest>`                                          | YYYY-MM-DD HH:MM:SS |

|                       |                                                                                |
|-----------------------|--------------------------------------------------------------------------------|
| Artifact type         | <media type or descriptor; Shape A: 'OCI image index'; Shape B: vnd.<...>+tar> |
| Aggregate index       | `<multi-arch-ref>` (also tagged `<aliases>`)                                   |
| Aggregate digest      | `sha256:<index-digest>`                                                        |
| Provenance            | SLSA Build Provenance v1, Sigstore-backed, pushed as OCI referrer              |
| Built by              | [Workflow run #<runid>](<workflow-url>)                                        |
| Built from commit     | [`<sha>`](<commit-url>)                                                        |
| Verify (CLI)          | `gh attestation verify oci://<ref> --repo <owner>/<repo>`                      |
| Release notes         | <link>                                                                          |
| Repo packages view    | <link>                                                                          |
```

Optional rows: `Tarball mirror` (Shape B's source-of-truth release), `Older PG majors` (per-PG-major artifacts), `Source bundle`, `Designation`, project-specific.

### 2.3 LATEST.md Is Attestation-Gated (binding)

`LATEST.md` MUST be written only by `update-latest-md.yml` (or equivalent), after `gh attestation verify` accepts every digest the block advertises. This is the same gate codified in `PROVENANCE.md` (this repo) and adopted across the fleet.

No bundle.yaml field or build flag bypasses the gate. If an artifact lacks attestation, it does not appear in `LATEST.md` — it appears as a placeholder block saying "no attested release yet". Period.

---

## 3. Updated `bundle.yaml` Schema (v0.3)

v0.3 adds two top-level arrays to the v0.2 schema. v0.2's `static_web:` stays as-is and is now equivalent to a `layer_sources` entry with `route` semantics.

### 3.1 New Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `layer_sources` | array of objects | no | Shape A upstreams to additively merge into the final image |
| `extracted_sources` | array of objects | no | Shape B upstreams to extract-and-copy at build time |
| `static_web` | array of objects | no | (v0.2-compatible) Shape A upstreams to mount as FastAPI/static-server routes |

A bundle may use any combination of the three.

### 3.2 `layer_sources[]` Object Schema

| Field | Type | Required | Description |
|---|---|---|---|
| `source_image` | string | yes | OCI image URI (Shape A). Tag or digest pin. |
| `into` | string | yes | Absolute path in the final image's filesystem where this layer's root is merged. Use `/` for full-root merge. |
| `attestation_repo` | string | yes | `<owner>/<repo>` for `gh attestation verify`. The build MUST fail if attestation does not verify. |
| `select` | array of string | no | Optional filename whitelist; if set, only matching paths are copied. Default: all paths. |
| `chown` | string | no | `uid:gid` to apply to copied files. Default: keep upstream. |

**Generator behavior:**
```dockerfile
FROM <source_image> AS <name>_layer
FROM <base>
COPY --from=<name>_layer <selectors> <into>
```

The build MUST run `gh attestation verify oci://<source_image> --repo <attestation_repo>` as a pre-build gate. A failed verify aborts the build before any push.

### 3.3 `extracted_sources[]` Object Schema

| Field | Type | Required | Description |
|---|---|---|---|
| `source_image` | string | yes | ORAS artifact URI (Shape B). Tag or digest pin. |
| `artifact_type` | string | yes | Expected media type, e.g. `application/vnd.styk.pgrdf.bundle.v1+tar`. The build MUST fail if the pulled artifact's `artifactType` does not match. |
| `extract` | array of objects | yes | List of `src_path` → `into` mappings to copy out of the unpacked tarball |
| `attestation_repo` | string | yes | `<owner>/<repo>` for `gh attestation verify`. |
| `strip_components` | integer | no | Passed to `tar -xz --strip-components=`. Default: 1. |

Each `extract` entry:

| Field | Type | Required |
|---|---|---|
| `src_path` | string | yes | Path inside the unpacked tarball (e.g. `lib/pgrdf.so`) |
| `into` | string | yes | Absolute path in the final image |

**Generator behavior:**
```dockerfile
FROM --platform=$BUILDPLATFORM ghcr.io/oras-project/oras:v1.2.2 AS <name>_fetch
ARG TARGETARCH
WORKDIR /work
RUN oras pull --output /work <source_image-with-arch-substitution> && \
    tar -xzf *.tar.gz --strip-components=<strip_components>
FROM <base>
# one COPY per extract entry:
COPY --from=<name>_fetch /work/<src_path> <into>
```

The build MUST verify the upstream attestation and the `artifactType` match before copying anything.

### 3.4 `static_web[]` (preserved from v0.2)

Unchanged. Equivalent to `layer_sources` with `into: /app/<route>` and an additional generator hook that mounts the directory in the FastAPI app (or static server). v0.2 examples continue to render correctly.

**The two CK.Lib.Js forms are interchangeable.** CK.Lib.Js v1.3.11 explicitly offers both. They produce identical browser-visible content at `/cklib`; pick by what the downstream Dockerfile prefers:

```yaml
# Form 1 — v0.2-compatible routed mount (static_web): mounted under /cklib by
# the FastAPI app / static server. Keeps the v0.2 routing semantics verbatim.
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.3.11
    route: /cklib
    attestation_repo: ConceptKernel/CK.Lib.Js

# Form 2 — additive-merge (layer_sources): CK.Lib.Js files land at a fixed
# filesystem path; the server is configured to serve that path at /cklib.
layer_sources:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.3.11
    into: /app/cklib/
    attestation_repo: ConceptKernel/CK.Lib.Js
```

Both are Shape A and both run the §4 attestation gate against `ConceptKernel/CK.Lib.Js`. CK.Lib.Js stays Shape A only — there is no Shape B / `extracted_sources` form for cklib (the whole payload is browser-loadable JS, so additive merge is the natural fit).

### 3.5 `spec_version`

Bundles MUST declare `spec_version: 0.3` at the top level once they adopt any v0.3-only field. v0.2 bundles continue to work; the generator detects by the highest field requested.

---

## 4. Build-Time Gates (binding)

Every build that consumes any upstream pin MUST gate on the following, in order. A failure at any gate aborts the build before publishing.

1. **Attestation verify** — for each `source_image` in `layer_sources`, `extracted_sources`, and `static_web`, run `gh attestation verify oci://<source_image> --repo <attestation_repo>`. Fail-fast on any rejection.
2. **Artifact type match** — for each `extracted_sources` entry, fetch the manifest's `artifactType` and compare against the declared one. Mismatch aborts.
3. **Digest pin recommended, tag pin permitted** — the spec allows either. A digest pin is preferred for reproducibility; a tag pin requires the consumer to accept whatever the tag currently resolves to (the attestation verify still binds it to a specific signed digest). A bundle composite pins every upstream by tag-or-digest, and the attestation verify is what binds the chosen tag to a single signed digest — so a tag pin in a composite is never unverified, it is verified-at-build-time.
4. **Per-arch coverage** — for `linux/amd64` AND `linux/arm64` per-platform leaves of Shape A images, BOTH must verify. For Shape B per-arch artifacts (e.g. `0.5.16-pg17-amd64` + `0.5.16-pg17-arm64`), both architecture leaves must verify.

**Live evidence pattern.** The attested CK.Lib.Js v1.3.11 release is the canonical Shape A index-leaf example a bundle composite verifies against:

```
index: sha256:3e6e4ab1569849005544dd6397033f479ab6ad0bc583979d1b25d56fc5301235
amd64: sha256:b42aab22cfba870ff0a862185f89924e0c3a879dce57c1f5e2b2f7677f7e106a
arm64: sha256:a3f038109dafb214f72c2401b8277c2d6d6bdd3b2149c995fd6ddd60f4c55461
verify: gh attestation verify oci://ghcr.io/conceptkernel/ck-lib-js:1.3.11 --repo ConceptKernel/CK.Lib.Js
```

Both `pgck-web` and `pgck` are likewise published both-arch-verified; a bundle that pins them MUST see `gh attestation verify` accept each per-platform leaf before any composite push.

---

## 5. Cross-Spec Alignment

This spec is one of three normative documents in this repo and binds with the other two:

- **`PROVENANCE.md`** — defines the attestation rules, `LATEST.md` write gate, and tag-cadence policy. v0.3 of this spec assumes every binding clause in PROVENANCE.md is in force.
- **`SPEC.NOTIFIES.v0.3`** (canonical at CK.Lib.Js) — defines the cross-repo coordination protocol. The "signal-for-migration" duty in §1.2 is discharged via SPEC.NOTIFIES.v0.3 frontmatter and naming.
- **`SPEC.OCIGERMI.TRACKS.DEVEL.v1.0`** — defines the production/devel split. v0.3 of this spec applies to BOTH tracks; devel-track artifacts MUST attest the same way production-track artifacts do. The only difference is which image name they land under.

---

## 6. Upstream Publisher Checklist

If your repo publishes OCI artifacts that other fleet repos consume, you MUST satisfy the following from this spec's `date:` forward.

- [ ] Every release goes through GitHub Actions only (Rule 1 of `PROVENANCE.md`).
- [ ] Every release ships `actions/attest-build-provenance@v1` per artifact (per-arch leaf AND aggregate index, if multi-arch).
- [ ] `LATEST.md` is updated by an `update-latest-md.yml` workflow gated on `gh attestation verify`.
- [ ] Every `LATEST.md` block includes the §2.2 fields.
- [ ] If you publish Shape B (tarball-in-OCI), document it in `LATEST.md` as `Artifact type: application/vnd.<org>.<project>.<class>.vN+tar` so consumers know to extract.
- [ ] If you publish Shape A, set the §2.1 manifest labels (especially `image.source`, `image.version`, `image.revision`, `image.created`).
- [ ] If you maintain multiple artifact lines (e.g. extension OCI + runtime image, like pgCK), each line gets its own `LATEST.md` block.

## 7. Downstream Consumer Checklist

If your repo composes upstream artifacts into bundles (currently: oci-germination):

- [ ] Pin upstream `source_image` by tag or digest.
- [ ] Declare each upstream in `bundle.yaml` under `layer_sources` (Shape A) or `extracted_sources` (Shape B) or `static_web` (Shape A with a route).
- [ ] Include `attestation_repo` on every declaration.
- [ ] When forced into `extracted_sources` for a payload you'd prefer additively (a `.so` library, etc.), send a NOTIFY to the upstream per SPEC.NOTIFIES.v0.3 with theme `additive-oci-shape-request` proposing a Shape A variant.
- [ ] Never bypass the attestation gate to ship a bundle. If upstream isn't attested, hold the pin and notify.

**Shape A request status (informational, non-blocking).** The pgRDF and pgCK extension leaves are Shape B today (tarball-in-OCI; the bundle extracts via `extracted_sources`). Parallel Shape A artifacts (`pgrdf-layer`, `pgck-layer`) are open `additive-oci-shape-request` NOTIFY proposals — future direction, not a blocker. Bundles use `extracted_sources` (Shape B) for these leaves today and migrate to `layer_sources` (Shape A) if/when the upstreams ship the parallel artifact. CK.Lib.Js is already Shape A; `pgck-web` is already Shape A (full FastAPI runtime image).

---

## 8. Worked Examples

Two worked bundles exercise the full v0.3 surface: §8.1 is the production-track static-cklib bundle (Go static server, no Python), §8.2 is the all-in-one dev bundle (FastAPI/pgck-web). §8.3 records the `pgck-web` consolidation and pin matrix both reference. Every entry below carries `attestation_repo` and `spec_version: 0.3`, so the §6/§7 checklists are exemplified, not only stated.

### 8.1 `bundle-pg17-pgrdf-pgck-static-cklib` (production variant — static, no Python)

This is the static-cklib bundle expressed under v0.3. Fields described in v0.2 stay the same; new fields make the upstream composition explicit. It serves `web/static/` and `/cklib` through the Go static server (`ociger-static-server`) — no FastAPI, no Python runtime. This is the v3.8-aligned production variant: browser ↔ NATS only, HTTP serves static assets.

```yaml
spec_version: 0.3
name: bundle-pg17-pgrdf-pgck-static-cklib
description: PostgreSQL 17 + pgRDF + pgCK + NATS + Go static server + CK.Lib.Js
image:
  registry: ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib
  base: ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.3
  runtime_profile: micro

# Shape B — extension OCI artifacts. Consumer extracts.
extracted_sources:
  - source_image: ghcr.io/styk-tv/pgrdf-bundle:0.5.16-pg17-${TARGETARCH}
    artifact_type: application/vnd.styk.pgrdf.bundle.v1+tar
    attestation_repo: styk-tv/pgRDF
    extract:
      - { src_path: lib/pgrdf.so, into: /usr/lib/postgresql/17/lib/pgrdf.so }
      - { src_path: share/extension, into: /usr/share/postgresql/17/extension }
  - source_image: ghcr.io/styk-tv/pgck:0.2.2-pg17-${TARGETARCH}
    # artifact_type CONFIRMED by pgCK (RESPONSE 2026-05-29): the published
    # manifest artifactType IS application/vnd.styk.pgck.extension.v1 (pushed by
    # release.yml via `oras push --artifact-type`; also shown in pgCK LATEST.md).
    artifact_type: application/vnd.styk.pgck.extension.v1
    attestation_repo: styk-tv/pgCK
    extract:
      - { src_path: lib/pgck.so, into: /usr/lib/postgresql/17/lib/pgck.so }
      - { src_path: share/extension, into: /usr/share/postgresql/17/extension }

# Shape A — static asset OCI image. Consumer additively merges (routed at /cklib).
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.3.11
    route: /cklib
    attestation_repo: ConceptKernel/CK.Lib.Js

ports:
  - { name: postgres, container_port: 5432 }
  - { name: static,   container_port: 8000 }
  - { name: nats,     container_port: 4222 }
  - { name: nats-wss, container_port: 9222 }
```

Generator output: a Dockerfile with two ORAS fetch stages (pgrdf, pgck), one Shape A stage (ck-lib-js), and a final base that COPYs each into its declared `into:`. All attestation verifies run before any image push.

### 8.2 `bundle-ck-allinone` (dev default — FastAPI / pgck-web, Shape A additive)

The all-in-one bundle composes the `pgck-web` FastAPI runtime additively. `pgck-web` is a full Python + uvicorn image (Shape A), so additive composition supplies the Python runtime that distroless-PG bases lack — this is what closes the "Python-missing-in-distroless" gap for this variant without a hand-edited venv step. The bundle pins the **consolidated `web/`** tree at `pgck-web/v0.2.4` (see §8.3 for why and which tags resolve).

```yaml
spec_version: 0.3
name: bundle-ck-allinone
description: CKP v3.8 all-in-one micro runtime — PG17 + pgRDF + pgCK + pgck-web (FastAPI) + CK.Lib.Js + NATS WSS
image:
  registry: ghcr.io/sporaxis-com/ociger-ck-allinone
  base: ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.3
  runtime_profile: micro

# Shape B — extension OCI artifacts (identical to §8.1).
extracted_sources:
  - source_image: ghcr.io/styk-tv/pgrdf-bundle:0.5.16-pg17-${TARGETARCH}
    artifact_type: application/vnd.styk.pgrdf.bundle.v1+tar
    attestation_repo: styk-tv/pgRDF
    extract:
      - { src_path: lib/pgrdf.so, into: /usr/lib/postgresql/17/lib/pgrdf.so }
      - { src_path: share/extension, into: /usr/share/postgresql/17/extension }
  - source_image: ghcr.io/styk-tv/pgck:0.2.2-pg17-${TARGETARCH}
    artifact_type: application/vnd.styk.pgck.extension.v1
    attestation_repo: styk-tv/pgCK
    extract:
      - { src_path: lib/pgck.so, into: /usr/lib/postgresql/17/lib/pgck.so }
      - { src_path: share/extension, into: /usr/share/postgresql/17/extension }

# Shape A — pgck-web FastAPI runtime image, additively merged.
# Supplies the Python runtime distroless-PG lacks (the FastAPI path).
layer_sources:
  - source_image: ghcr.io/styk-tv/pgck-web:v0.2.4
    into: /              # full-root merge: Python + uvicorn + web/ app
    attestation_repo: styk-tv/pgCK

# Shape A — CK.Lib.Js, routed at /cklib (interchangeable with layer_sources; see §3.4).
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.3.11
    route: /cklib
    attestation_repo: ConceptKernel/CK.Lib.Js

ports:
  - { name: postgres, container_port: 5432 }
  - { name: fastapi,  container_port: 8000 }
  - { name: nats,     container_port: 4222 }
  - { name: nats-wss, container_port: 9222 }
```

Generated Shape A composition for `pgck-web`:

```dockerfile
FROM ghcr.io/styk-tv/pgck-web:v0.2.4 AS pgckweb_layer
FROM <base>
COPY --from=pgckweb_layer / /
# supervisor then starts: uvicorn web.app:app  (+ the PG launcher + NATS)
```

**Consumption-path decision (per the CKE-4 NOTIFY ask).** oci-germination declares:

- **`bundle-ck-allinone` = FastAPI-supervised (dev default).** It additively merges the `pgck-web` Shape A image and supervises `uvicorn web.app:app` alongside the PG launcher. This is the variant that exercises the full FastAPI surface.
- **`bundle-pg17-pgrdf-pgck-static-cklib` = static-only (production).** Go static server, no Python; serves `web/static/` + `/cklib`. The Display page is NATS-driven and needs no API calls, so it works static-only. This is the v3.8-aligned production variant.

### 8.3 `pgck-web` consolidation and resolvable-tag matrix

pgCK consolidated `web_demo/` **into** `web/`. The directory name stays `web/`; the content is now the v3.8 dual-page Display/Board surface plus CK.Lib.Js v1.3-aligned CKClient wiring. The legacy single-page `web/` tree (from `pgck-web/v0.1.0`) no longer exists at HEAD. Bundles that still pin `pgck-web/v0.1.0` should advance to the consolidated tree. All listed tags remain resolvable (git tags are immutable):

| Tag | Content |
|---|---|
| `pgck-web/v0.1.0` | legacy single-page `web/` (original layout) |
| `pgck-web/v0.2.0` | final `web_demo/`-as-content snapshot (immediately pre-consolidation) |
| `pgck-web/v0.2.1` | first consolidated `web/` (rename + import-path rewrite) |
| `pgck-web/v0.2.2` | interim |
| `pgck-web/v0.2.3` | CKClient v1.3 aligned, dual-page, NATS-broadcast-render verified |
| `pgck-web/v0.2.4` | **recommended pin** — current attested head; pgCK confirmed `web/` + `uvicorn web.app:app` + port 8000 as stable contracts |

`uvicorn web_demo.app:app` is replaced by `uvicorn web.app:app`. The Python-in-distroless concern is decoupled from pgCK's layout: pgCK ships one `web/` tree regardless of bundle variant; FastAPI-vs-static is the oci-germination policy choice recorded in §8.2.

---

## 9. Validation Rules (binding)

The generator and any conformance tooling MUST enforce:

- [ ] `spec_version` is `0.3` if any v0.3-only field is used.
- [ ] Every `source_image` is a valid OCI reference.
- [ ] Every entry declares an `attestation_repo`.
- [ ] `gh attestation verify` succeeds for every declared `source_image` (against `attestation_repo`) before the build proceeds.
- [ ] `extracted_sources[*].artifact_type` matches the upstream manifest's `artifactType`.
- [ ] No two `static_web[*].route` values collide.
- [ ] No two `layer_sources[*].into` or `extracted_sources[*].extract[*].into` paths collide in a way that overwrites a previously-merged file (unless `select` filters it out).
- [ ] Multi-arch upstreams resolve per-platform on both `linux/amd64` and `linux/arm64`.

---

## 10. Backwards Compatibility

- v0.2 bundles continue to work. Their `static_web:` blocks render identically under v0.3.
- v0.1 bundles continue to work. They declare no upstream consumption beyond what's in their hand-edited Dockerfile.
- Once a bundle declares any v0.3-only field, it MUST set `spec_version: 0.3` and conform to §4's gates.

---

## 11. Authoritative Distribution

This file lives in `oci-germination/SPEC.OCI.BUNDLE.v0.3.md` and is canonical. Sister repos may reference it by URL. Updates land here first; any changes propagate via SPEC.NOTIFIES.v0.3 NOTIFY threads from this repo outward.

---

## 12. References

- [`SPEC.OCI.BUNDLE.v0.1.md`](./SPEC.OCI.BUNDLE.v0.1.md) — original schema.
- [`SPEC.OCI.BUNDLE.v0.2.md`](./SPEC.OCI.BUNDLE.v0.2.md) — `static_web` introduction.
- [`PROVENANCE.md`](./PROVENANCE.md) — attestation policy.
- [`SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md`](./SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md) — production/devel tracks.
- [`SEMANTIC-VERSIONING.md`](./SEMANTIC-VERSIONING.md) — tag-shape conventions.
- `SPEC.NOTIFIES.v0.3` (canonical at `/Users/neoxr/git_conceptkernel/CK.Lib.Js/_WIP/SPEC.NOTIFIES.v0.3.md`) — cross-repo coordination.
- OCI Image Spec — https://github.com/opencontainers/image-spec
- ORAS — https://oras.land
- SLSA Build Provenance v1 — https://slsa.dev/spec/v1.0/provenance
