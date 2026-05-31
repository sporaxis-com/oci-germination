---
title: "SPEC.OCI.BUNDLE.v0.4 — Authoritative Packaging Protocol for the Sporaxis-Com / ConceptKernel Fleet"
version: 0.4
date: 2026-05-31
status: Authoritative
supersedes: SPEC.OCI.BUNDLE.v0.3 (2026-05-29)
audience: Every repo in the fleet that publishes OCI artifacts consumed by another fleet repo — currently oci-germination, pgRDF, pgCK (extension + pgck-web), CK.Lib.Js, and any future sibling.
changes-from-v0.3:
  - "§1.4 NEW — Shape A 'Delta' composition contract: prod images on `scratch` final, supervised by `s6-overlay`, web served by `busybox httpd` (or equivalent statically-linked binary). Bespoke supervisors are deprecated in favour of OS-native primitives."
  - "§2.4 NEW — Manifest labels `ck.bundle.role` and `ck.bundle.never-prod` for unambiguous prod/devel classification."
  - "§4.4 NEW — Build-time gate: no Python, no FastAPI, no /opt/venv in any prod image. The bench/devel sibling is the SOLE sanctioned home for Python."
  - "§5 NEW — Sibling-not-sidecar rule: Python/benchmark/devel variants live as a SEPARATE OCI image that runs alongside the prod image over the shared network, never embedded as a profile of the same image."
  - "§6 NEW — `oci-germination` (and any layer-addition repo) MUST NOT compile upstream code. Only thin in-tree static binaries + composition by FROM/COPY of pre-built upstream artifacts."
  - "§7 NEW — Inbound NOTIFIES scan as a hard pre-condition for any cross-repo touch. References SPEC.NOTIFIES.v0.3."
  - "§8 NEW — Fleet-wide base-release contract opening: upstream authorities (pgRDF for pg-base SONAMEs; pgCK pgck-web for sibling-bench userspace) MUST declare their compile target's Debian release in a manifest annotation. Closes the bookworm/trixie SONAME-collision class."
  - "§9 RENAMED from v0.3 §8 — Worked examples updated for the Delta composition (ck-allinone v0.7.x is now the canonical example)."
  - "§10 NEW — `_WIP/` is confidential across all fleet repos; never reference paths under `_WIP/` in any committed file. Public docs cite specs by name without linking to draft paths."
---

# SPEC.OCI.BUNDLE.v0.4 — Authoritative Packaging Protocol

> **Supersedes v0.3.** The v0.3 §1–3 artifact shapes (Shape A/B/C), §2.2 LATEST.md metadata contract, §2.3 attestation-gate, §3 bundle.yaml schema, §4 build-time gates, §6 publisher checklist, §7 consumer checklist all carry forward unchanged unless explicitly amended below. v0.3 remains a valid historical reference; this document is what new releases conform to.

## Scope and binding

Every OCI artifact published by a fleet repo and consumed by another fleet repo MUST satisfy this spec. The fleet today: oci-germination, pgRDF, pgCK (extension + pgck-web), CK.Lib.Js, and any future sibling that joins the same NOTIFIES coordination space.

---

## 1. Recognized Artifact Shapes (unchanged from v0.3 §1)

Shape A (filesystem-layer OCI image), Shape B (tarball-in-OCI), Shape C (ORAS general). v0.3's §1.1–1.3 definitions hold verbatim.

### 1.4 NEW — Shape A "Delta" composition contract (binding for prod images)

When a Shape A image is intended for production (not bench/devel; see §2.4 + §5) it MUST be composed using ONLY the following materials:

| Material | Examples | Acceptable in prod? |
|---|---|---|
| Pre-attested upstream OCI base (single FROM) | `postgres:17-bookworm`, `scratch + selectively-copied bookworm libs` | YES |
| Pre-attested fleet-shipped composed base (FROM) | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.6` | YES |
| Pre-built upstream static binary added via `COPY --from` | `busybox:1.36.1-musl`, `s6-overlay v3.2.3.0` (tarball extracted in build stage) | YES |
| Pre-attested fleet static asset added via `COPY --from` | `ghcr.io/conceptkernel/ck-lib-js:1.3.11` | YES |
| Tiny in-tree static binary built in build stage (Go static link only) | `ociger-pg-launcher`, `ociger-static-server`, `ociger-supervisor` | YES — but prefer `s6-overlay` for new supervision needs (see §1.4.2) |
| ────── | ────── | ────── |
| OS package install (`apt-get install`, `apk add`, `yum install`) in final image to satisfy any binary's NEEDED list | n/a | **NO** — see §4.4 |
| Compile upstream source in final image (or in a build stage that emits to the final image) | `cargo build`, `make install` of an upstream extension | **NO** — see §6 |
| Python runtime, virtualenv, or any interpreter for a long-running prod process | `python:3.11-slim`, `/opt/venv`, `uvicorn`, `node`, `ruby`, … | **NO** — see §4.4 + §5 |

#### 1.4.1 Recommended final-image base

Prod images SHOULD use `scratch` (or an equivalent that adds zero unused userspace). If a true scratch base is impractical for a specific bundle, distroless variants (`gcr.io/distroless/static`, `gcr.io/distroless/base`) are acceptable — but the rationale MUST be recorded in the bundle.yaml.

#### 1.4.2 Recommended supervisor

Multi-process prod images SHOULD use `s6-overlay` as PID 1. Bespoke supervisors (e.g. in-tree Go supervisors with hand-rolled signal handling, process restart, dependency ordering) are deprecated for new bundles. Existing bundles using bespoke supervisors continue to ship; new bundles or major-version rewrites adopt `s6-overlay`.

Rationale: `s6-overlay` is widely audited, statically linked, scratch-compatible, and offloads the supervision concern to upstream maintenance. Bespoke supervisors create per-fleet maintenance burden for a well-solved problem.

#### 1.4.3 Recommended HTTP serving (when prod needs HTTP)

If a prod image serves static HTTP, it SHOULD use `busybox httpd` (single static binary, ~1 MB, suitable for marketplace size budgets) or `nginx` (if config-language / TLS / range requests are required). Bespoke HTTP servers are acceptable but again deprecated for new bundles for the same reason as §1.4.2.

---

## 2. Per-Artifact Metadata Contract

§2.1 (OCI manifest labels), §2.2 (LATEST.md block per release), §2.3 (LATEST.md is attestation-gated) carry forward from v0.3 unchanged.

### 2.4 NEW — Role and prod/devel manifest labels (binding for ALL images)

Every Shape A image MUST carry the following OCI manifest labels:

```
ck.bundle.role            = prod | devel | bench
ck.bundle.never-prod      = true | false
```

Semantics:

- `ck.bundle.role` declares the intended use:
  - `prod` — consumers may pin this image as their production runtime.
  - `devel` — for development workflows (e.g. local dev with extra tooling).
  - `bench` — for benchmark or load-test workflows; runs alongside a prod image as a sibling (see §5).
- `ck.bundle.never-prod`:
  - `false` for `prod` images.
  - `true` for `devel` and `bench` images. Tooling MAY refuse to pull or run a `never-prod=true` image into a production-tagged environment.

Per-bundle .yaml mirror (REQUIRED):

```yaml
role: prod | devel | bench
never_prod: true | false
```

The bundle.yaml fields are authoritative for tooling that doesn't pull manifests; the manifest labels are authoritative for tooling that does.

### 2.5 LATEST.md per-bundle row MUST surface the role

The bundle's LATEST.md block MUST include a `Role` row alongside the existing metadata rows. If `role=bench` or `role=devel`, the block MUST also include a `Production use` row with the value `Not for production`.

---

## 3. `bundle.yaml` Schema (extends v0.3 §3)

All v0.3 §3 fields hold. Two new required top-level fields:

```yaml
role: prod | devel | bench         # mirrors the manifest label (REQUIRED)
never_prod: true | false           # mirrors the manifest label (REQUIRED)
```

And one new section, conditional on `role: bench`:

```yaml
bench_target:                       # REQUIRED when role: bench
  image: ghcr.io/sporaxis-com/ociger-ck-allinone   # what this bench image talks to
  default_network: ociger-ck-allinone-net          # docker network the sibling joins
  defaults:                          # env vars that point at the target
    CK_PG_HOST: ck-allinone
    CK_NATS_URL: nats://ck-allinone:4222
```

---

## 4. Build-Time Gates (extends v0.3 §4)

v0.3's §4.1–4.3 (regenerate bundle outputs, lint preload contract, smoke before publish) hold.

### 4.4 NEW — Prod images are Python-free (binding)

For every bundle with `role: prod`, the CI publish pipeline MUST include a step that asserts the published image contains NO Python interpreter, no `/opt/venv`, no `uvicorn`, no `fastapi`. Reference assertion script: `scripts/smoke-ck-allinone.sh` §⑥.

Equivalent check (`busybox find` inside the published image):

```sh
docker run --rm --entrypoint /bin/busybox "$IMAGE" find / \
  \( -name 'python*' -o -name 'uvicorn*' -o -name 'fastapi*' -o -path '*/opt/venv*' \) \
  -type f 2>/dev/null | wc -l
# Must return 0
```

A non-zero result aborts the release. Python-bearing variants live as sibling bundles per §5.

### 4.5 NEW — Build-time gate: NO apt-install of libs to satisfy NEEDED of upstream binaries

If a prod bundle's Dockerfile contains `apt-get install` / `apk add` / equivalent in its FINAL stage, the install list MUST be limited to userspace bits that the bundle itself adds (e.g. `ca-certificates` for a network client we ship). It is a violation to apt-install libraries to satisfy the dynamic linker for upstream-compiled binaries (e.g. installing `libicu76` to make a bookworm-compiled `postgres` binary load in a trixie userspace). The fix for such SONAME mismatches is upstream coordination (§8), not local shims.

---

## 5. NEW — Sibling-not-sidecar rule (binding)

Bench/devel variants that need Python (or any other heavyweight runtime that violates §1.4 / §4.4) live as a **SEPARATE OCI image** — never as a profile or sidecar of the same prod image. The sibling image:

- Has a distinct GHCR name (e.g. `ociger-pgck-bench` next to `ociger-ck-allinone`).
- Declares `role: bench`, `never_prod: true`, `bench_target: ...` per §3.
- At runtime joins the same docker network as the prod image and reaches it over the wire (NATS WSS, pg wire, HTTP) — not via shared filesystem or shared process tree.

Rationale: marketplace cost discipline. Prod images carry only what production needs; bench/devel tooling does not contaminate the prod image's RSS, CVE surface, or pull-time. The sibling pattern keeps each image's role legible and its lifecycle independent.

Reference implementation: `bundle-pgck-bench` (`role: bench`, `never_prod: true`, targets `ck-allinone`).

---

## 6. NEW — Layer-addition-only repos MUST NOT compile upstream code

Repos whose role is to compose pre-built fleet/upstream artifacts (today: `oci-germination`) MUST NOT compile upstream source into their published images. Allowed in such repos:

- `FROM <pre-attested upstream image>`
- `COPY --from=<pre-attested upstream image>` to add static binaries / asset trees
- In-tree Go static-link builds in a build stage that emit single binaries (no dynamic deps)
- Configuration files (s6 service defs, nginx.conf, etc.) added via COPY

Not allowed:

- `oras pull` + `tar -xz` + `COPY` of upstream source/binaries when a pre-composed image alternative exists upstream. (Tolerated as a transitional measure when no pre-composed image is yet available, per the [[only-forward-never-revert]] discipline.)
- `cargo build`, `make`, `go build` of upstream-owned code paths
- `apt-get install postgresql-17` to install upstream-owned binaries we should be inheriting from an attested base

Rationale: compilation is the upstream's responsibility — they own the SONAMEs, the attestation, the security patches. A layer-addition repo that compiles upstream code becomes responsible for maintenance it didn't sign up for and breaks the attestation chain (the fleet image's attestation no longer reaches back to the upstream's audited build).

---

## 7. NEW — Inbound NOTIFIES scan as cross-repo pre-condition (binding)

Before any cross-repo work — bumping a pin, adapting to upstream behaviour, designing a smoke around an upstream contract, writing this kind of spec — the agent / human responsible MUST scan all related repos' `_WIP/` directories for inbound NOTIFIES targeting their repo. References `SPEC.NOTIFIES.v0.3`.

Operative discipline (per practice 2026-05-31):

```sh
for d in <each related fleet repo>/_WIP; do
  [[ -d "$d" ]] && (
    cd "$d"
    for f in NOTIFIES.<our-repo>.*.md; do
      [[ "$f" == *-RESPONSE.md ]] && continue
      base="${f%.md}"
      [[ -f "${base}-RESPONSE.md" ]] || echo "$d/$f"
    done
  )
done
```

Any unanswered inbound NOTIFY MUST be RESPONDed to (adjacent to source in the foreign `_WIP/`) before proceeding with the work. Acting past an unanswered inbound is a violation.

---

## 8. NEW — Fleet-wide base-release contract (forthcoming)

The bookworm/trixie SONAME-collision class (downstream consumer pulls a base built on one Debian release into a userspace from another release; dynamic linker fails) is a recurring fleet-level issue. Each component that ships compiled binaries (pgRDF, pgCK, postgres-base) MUST publish its target Debian release in a manifest annotation:

```
ck.compile.target.os         = debian-bookworm | debian-trixie | alpine-3.20 | ...
ck.compile.target.soname     = libicu72 | libicu76 | ...  (when relevant)
```

Consumers can then refuse cross-release composition at build time. The exact annotation schema is TBD; this section opens the conversation. **pgRDF is the de-facto authority for pg-base's target release** (its `.so` compile target dictates the SONAMEs the whole pg-bundle chain inherits — see oci-germination's `project_pgrdf_owns_pg_base_release` discipline). A cross-fleet NOTIFY will solicit the binding annotation values.

Until §8 lands as binding, the operational rule is: when composing across fleet images, manually verify SONAME compatibility (one base release across the FROM chain). The Delta composition of `ck-allinone v0.7.x` achieves this by NOT inheriting any non-pg_base userspace at all.

---

## 9. Worked Examples (updates v0.3 §8)

### 9.1 `bundle-ck-allinone` Delta (canonical example for v0.4 compliance)

```yaml
role: prod
never_prod: false
spec_version: 0.4
image:
  registry: ghcr.io/sporaxis-com/ociger-ck-allinone
  base_image: ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro
  final_image: scratch
supervisor:
  binary: s6-overlay v3.2.3.0
components:
  busybox: { version: 1.36.1, role: httpd-applet }   # §1.4.3
  cklib:   { version: 1.3.11 }                       # static asset
```

Dockerfile composition follows §1.4: FROM `pg_base` (scratch + selectively-copied bookworm), COPY s6-overlay tarball, COPY `/bin/busybox`, COPY cklib, COPY s6 service defs. No apt-install. No Python. ENTRYPOINT `/init`.

Smoke (§4.4) asserts:
- postgres + pgRDF + pgCK create-extension succeed
- NATS core + WSS bridge listen
- busybox httpd serves `/cklib/*`
- `find / -name python* -o ...` returns 0
- PID 1 is `s6-svscan`

### 9.2 `bundle-pgck-bench` sibling (canonical example for §5 compliance)

```yaml
role: bench
never_prod: true
spec_version: 0.4
image:
  registry: ghcr.io/sporaxis-com/ociger-pgck-bench
  base_image: ghcr.io/styk-tv/pgck-web   # FROM pgck-web; runtime is Python+FastAPI
bench_target:
  image: ghcr.io/sporaxis-com/ociger-ck-allinone
  default_network: ociger-ck-allinone-net
  defaults:
    CK_PG_HOST: ck-allinone
    CK_NATS_URL: nats://ck-allinone:4222
```

Runs **alongside** a prod ck-allinone on the same docker network. Never bundled inside ck-allinone, never used in prod.

### 9.3 Other bundles

`bundle-pg17-pgrdf-pgck-static-cklib`, `bundle-pg17-pgrdf-pgck-nats-micro` and the rest continue under the v0.3 worked-example shapes; they'll migrate to §1.4 Delta composition on next major rewrite (no urgency — they're already Python-free).

---

## 10. NEW — `_WIP/` confidentiality (binding for every fleet repo)

Every fleet repo has a `_WIP/` directory (gitignored) that holds:
- inbound and outbound NOTIFIES per SPEC.NOTIFIES.v0.3
- draft specs not yet promoted to a public root
- triage notes, audit reports, decision memos

**No committed file in any fleet repo may reference any `_WIP/` path** — not by absolute path, not by URL (`https://github.com/.../blob/main/_WIP/...`), not by relative path in prose. The only acceptable mention is in `.gitignore` itself.

Public docs that need to cite a draft spec MUST name the spec without linking to its `_WIP/` location. Readers who need access ask the maintainer. Once a spec promotes to a public repo root (or to a public URL), it MAY be linked normally.

Pre-commit reflex (every repo):

```sh
grep -rIn --exclude-dir=.git --exclude-dir=_WIP \
  --exclude-dir=node_modules '_WIP' .
# Acceptable: exactly one hit in .gitignore. Anything else is a violation.
```

---

## 11. Cross-Spec Alignment

Replaces v0.3 §5:

- `SPEC.NOTIFIES.v0.3` (authored in CK.Lib.Js) — cross-repo coordination protocol.
- `SPEC.OCIGERMI.TRACKS.DEVEL.v1.0` — devel vs prod tracks (oci-germination side).
- `PROVENANCE.md` (per repo) — attestation policy.

---

## 12. Migration from v0.3

| If you have… | Action under v0.4 |
|---|---|
| A prod bundle that passes v0.3 §4 + §6 checks | Continue shipping; add `role: prod` + `never_prod: false` to bundle.yaml and the manifest labels on the next release. |
| A prod bundle that bundles Python or FastAPI | Migrate to Delta composition (§1.4); split the Python part out as a `role: bench` sibling per §5. `ck-allinone v0.6.x → v0.7.x` is the worked example. |
| A bench/devel bundle currently shipped on the prod image name | Rename to a `-bench` / `-devel` GHCR repo; add the `role` + `never_prod` labels; document the sibling pattern. |
| Specs / docs / scripts that link to `_WIP/` paths | Remove per §10. |
| A repo that compiles upstream code outside of §6 allowances | Plan the migration; coordinate via NOTIFY to upstream for a pre-composed image. |

---

## 13. Acknowledgements

This spec landed on the back of two cross-repo discipline lessons learned 2026-05-29 → 2026-05-31:

1. **The pgRDF v0.5.1-stuck-label story** (closed v0.5.25, validated through v0.5.28) demonstrated that upstream-side discipline (Cargo.toml ↔ control ↔ META.json ↔ compose alignment, plus 4-CI-gate prevention) is the right place to fix that bug class — downstreams who tried to work around it locally only made tech debt. v0.4 §4.5 codifies the no-shim rule that follows from that.

2. **The ck-allinone v0.6.6 trixie/bookworm SONAME collision** demonstrated that "FROM upstream + COPY pg_base binaries on top" only works when both sides target the same Debian release. v0.4 §8 opens the fleet-wide base-release contract that would prevent the recurrence; v0.4 §1.4 Delta composition is the operational workaround in the interim (no foreign userspace = no SONAME collision).

Future v0.5+ work, likely: SHACL formalisation of the manifest labels + bundle.yaml schema (so violations become machine-checkable, not human-reflex), per the broader fleet direction of NOTIFIES + manifest contracts moving onto pgCK-governed RDF.
