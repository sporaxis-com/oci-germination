---
title: "SPEC.OCIGERMI.TRACKS.DEVEL.v1.0 — Parallel Production & Devel Tracks for oci-germination Bundles"
version: 1.0
date: 2026-05-28
status: Stable
audience: Bundle authors, downstream pinners (pgCK-SKU, nx-cluster-v4, ConceptKernel), and CI maintainers
---

# SPEC.OCIGERMI.TRACKS.DEVEL.v1.0 — Parallel Tracks

This specification defines how `oci-germination` publishes two parallel image lineages per bundle: a **production track** consumers can safely pin, and a **devel track** carrying new features that are not yet advertised as ready. The two tracks coexist and evolve independently until a devel feature set is promoted to production.

---

## 1. Why Two Tracks

Bundles in this repo carry runtime-critical software (PostgreSQL, pgRDF, pgCK, NATS, FastAPI/static servers, CK.Lib.Js). Several constraints make a single linear release line painful:

- **Downstream pinners need a stable target.** pgCK-SKU, nx-cluster-v4 deployments, and consumers cannot accept a version-bump that introduces an unverified change.
- **Upstream churn forces frequent rebuilds.** CK.Lib.Js, pgRDF, pgCK ship new versions on their own cadence. We want to absorb those changes promptly without destabilizing what's deployed.
- **Latent gaps surface during development.** The FastAPI-in-distroless bug (Task #12, deleted in favor of static-cklib) is a recent example: a published image looked fine for months before the gap surfaced. We need a place to land work-in-progress without consumers accidentally pinning it.
- **Spec-side coordination is in motion.** SPEC.OCI.BUNDLE.v0.2's generator gap (`components:` / `static_web:` not honored by `ociger-gen`) is an active workstream. Generator-rendered bundles must be testable alongside today's hand-crafted ones.

Two parallel tracks give us a place to do both: a calm production line and a working devel line.

---

## 2. Track Definitions

### 2.1 Production Track

| Property | Value |
|---|---|
| Image name | `ghcr.io/sporaxis-com/ociger-<bundle>` (no suffix) |
| Tag format | `vMAJOR.MINOR.PATCH` per `SEMANTIC-VERSIONING.md` |
| Audience | Downstream production pinners |
| Stability | Advertised ready; smoke-verified; no known critical latent gaps |
| Cadence | Slow; promoted from devel only after explicit go-ahead |
| Branch | `main` (tags `release-<bundle>-vM.N`) |

**Current production heads** (as of this spec's `date:`):

| Bundle | Production tag |
|---|---|
| `ociger-ck-allinone` | Delta — see LATEST.md (s6-overlay + busybox httpd, Python-free, marketplace-minimal) |
| `ociger-pg17-pgrdf-pgck-static-cklib` | see LATEST.md (ociger-static-server, Python-free) |

> **2026-05-31 — `ociger-pg17-pgrdf-pgck-web-cklib` retired.** That bundle bundled FastAPI/uvicorn/pgck-web in-image; the same web layer needs are now met by `static-cklib` (no Python at all) for static deployments, and by the sibling `ociger-pgck-bench` (Python+FastAPI lives in a separate sidecar) for benchmarking against prod containers. The "FastAPI-in-distroless gap" referenced throughout this spec is **closed** by the ck-allinone Delta migration; historical references are kept below for context.

### 2.2 Devel Track

| Property | Value |
|---|---|
| Image name | `ghcr.io/sporaxis-com/ociger-<bundle>-devel` |
| Tag format | `vMAJOR.MINOR.PATCH` (parallel semver, independent of production) |
| Audience | Bundle authors, integration testers, feature-development bots |
| Stability | Work-in-progress; may have known gaps; not for production pinning |
| Cadence | Fast; new builds land on every meaningful change |
| Branch | `main` (tags `release-<bundle>-devel-vM.N`) |

The `-devel` suffix is part of the **image name**, not a tag suffix. This makes accidental pinning impossible: a consumer that types `ociger-ck-allinone` will never get a devel image, and a consumer that types `ociger-ck-allinone-devel` is opting in explicitly.

**Why not a tag suffix** (e.g. `v0.6.0-devel`): tag suffixes mix prod and devel under one repository name, which breaks `:latest` semantics and confuses downstream tooling that expects a clean semver. The name-suffix approach keeps each track's tag history linear and inspectable.

---

## 3. Tagging Conventions

### 3.1 Tag Naming (Git)

| Track | Git tag pattern | Container tag pattern |
|---|---|---|
| Production | `release-<bundle>-vM.N` | `vM.N.K` (K from `git describe --tags`) |
| Devel | `release-<bundle>-devel-vM.N` | `vM.N.K` (K from `git describe --tags --match release-<bundle>-devel-*`) |

Both follow the 2-number-tag + git-distance-patch scheme from `SEMANTIC-VERSIONING.md`. They share the same machinery; the only difference is the bundle name slot in the tag.

### 3.2 Tag Coexistence

The two tracks may carry **different** semver versions at the same time:

```
ociger-ck-allinone:v0.5.1                  ← production (stable)
ociger-ck-allinone-devel:v0.6.3            ← devel (new features, in flight)
```

There is no requirement that devel's MAJOR.MINOR must be ahead of production's. When a devel feature lands, devel typically jumps ahead of production. After a promotion (§5), production may briefly equal devel before devel moves on.

### 3.3 Semantic Versioning Independence

Each track increments its own semver independently. A devel `v0.6.x` does not constrain production's next version — production may go `v0.5.1 → v0.7.0` if the devel work warrants it, or `v0.5.1 → v0.5.2` if production only absorbs a small fix.

---

## 4. CI/CD Workflow

### 4.1 Production Releases

```yaml
on:
  push:
    tags:
      - 'release-<bundle>-v*'                # production tags
      - '!release-<bundle>-devel-v*'         # explicitly NOT devel
```

Builds `ghcr.io/sporaxis-com/ociger-<bundle>:vM.N.K`.

### 4.2 Devel Releases

```yaml
on:
  push:
    tags:
      - 'release-<bundle>-devel-v*'
```

Builds `ghcr.io/sporaxis-com/ociger-<bundle>-devel:vM.N.K`.

### 4.3 `:latest` Tag Policy

- `ociger-<bundle>:latest` → moves with production releases only.
- `ociger-<bundle>-devel:latest` → moves with devel releases.

A consumer who pulls `:latest` on the production image will never see a devel artifact.

### 4.4 Workflow Implementation Note

`.github/workflows/build-bundles.yml` already supports per-bundle release tags with semver derivation. Extending it for devel requires:

1. Add `release-<bundle>-devel-v*` patterns to the `on.push.tags` array.
2. In the "Extract bundle name and version" step, detect the `-devel-` infix in the tag and append `-devel` to the bundle name when constructing the container image.
3. Scope `git describe --tags --match` to the same prefix (`release-<bundle>-devel-*` vs `release-<bundle>-v*`).

---

## 5. Promotion (Devel → Production)

A devel image is promoted to production when **all** of the following are true:

- Smoke tests pass on every supported platform (`linux/amd64`, `linux/arm64`).
- All known critical gaps are closed (see §6 for what counts as "critical").
- No `NOTIFIES.*.md` thread from a consuming repo is awaiting a blocker-severity reply.
- A release note exists in the bundle's `RELEASE_NOTES.md` (or commit body) documenting what advanced since the last production cut.
- The "Current Production Heads" table in §2.1 of this spec is updated in the same commit that cuts the production tag.

Promotion mechanics:

1. Tag `release-<bundle>-vM.N` on the same commit that's already in production-ready state on devel.
2. CI builds and pushes `ociger-<bundle>:vM.N.K`.
3. Update §2.1 of this spec, commit, push.
4. Optionally write a NOTIFY to downstream pinners signaling the new production head.

Promotion does **not** delete or rename the devel image at that version. Devel continues from whatever state it's in.

---

## 6. What Counts as "Critical"

A gap is **critical** (blocks promotion) when any of the following is true:

- A documented bundle capability does not function (e.g. FastAPI never starting in `ck-allinone:v0.6.0` blocks promotion of `v0.6.x` to production).
- A core extension fails to initialize (PgAtomic not initialized → blocks).
- An upstream `-RESPONSE.md` requested a downstream fix and the fix isn't in.
- Smoke script for the bundle does not exist or does not run.

A gap is **non-critical** (doesn't block promotion) when:

- It's a documentation drift (e.g. README points at an older version).
- It's a stylistic concern flagged by hadolint/golangci-lint at warning level.
- It's a future-architecture migration (e.g. ck-allinone still on `pgCK/web/` while `web_demo/` is preferred — flagged but not blocking).

---

## 7. Pinning Guidance for Consumers

Downstream consumers (pgCK-SKU, nx-cluster-v4, dev workstation `compose.yml` files, etc.):

- **Pin production** by default. Use the image name without `-devel`. Use a specific `vM.N.K` tag, not `:latest`, for deployments.
- **Pin devel** only when you are intentionally integration-testing a forthcoming feature, or you are the bundle author iterating.
- **Never mix** in a single environment. If you have a `ck-allinone` deployment, don't have one container on `ociger-ck-allinone:v0.5.1` and another on `ociger-ck-allinone-devel:v0.6.3` — pick a track per environment.

Reference table (always confirm against §2.1 before pinning):

```yaml
# Production pin (stable)
image: ghcr.io/sporaxis-com/ociger-ck-allinone:v0.5.1

# Devel pin (bleeding edge, integration testing only)
image: ghcr.io/sporaxis-com/ociger-ck-allinone-devel:v0.6.3
```

---

## 8. Lifecycle & Retention

### 8.1 Production Tags

- Permanent. Never deleted. Never overwritten. Once pushed, the digest is immutable.
- The `:latest` alias may move forward, but old `vM.N.K` tags remain pullable.

### 8.2 Devel Tags

- Same as production: permanent and immutable per tag, no overwriting.
- The `-devel:latest` alias moves forward with each devel push.
- Devel images that pre-date a successful promotion remain pullable for historical reference.

### 8.3 Archived Tracks

When a major version bump occurs (e.g. moving from `v0.X` to `v1.X` line on production), the old major's most-recent tag remains as the long-tail support point. No `:latest-v0` alias is created automatically; if you need long-tail tracking, pin a specific `vM.N.K`.

---

## 9. NOTIFIES Protocol Interaction

Per SPEC.NOTIFIES.v0.3, cross-repo coordination messages reference specific images. When writing a NOTIFY:

- **Be explicit about track**: always include the full image name (`ociger-<bundle>` vs `ociger-<bundle>-devel`) so the recipient knows which lineage you're discussing.
- A NOTIFY about a devel feature that's not yet on production is fine — that's part of how coordination flows. But state clearly that the recipient should not promote the feature to their own production pinning until you signal readiness.

Example frontmatter snippet:

```yaml
about-library: oci-germination
about-version: ociger-ck-allinone-devel:v0.6.3 (devel track; not advertised ready)
```

---

## 10. Current State as of This Spec

| Bundle | Production head | Devel head |
|---|---|---|
| `ociger-ck-allinone` | Delta (s6-overlay + busybox httpd) — see LATEST.md | n/a — Delta is the prod path; Python-bearing benchmark variant lives separately as `ociger-pgck-bench` |
| `ociger-pg17-pgrdf-pgck-static-cklib` | see LATEST.md | n/a |

The track distinction for ck-allinone collapsed in 2026-05-31's Delta migration: the prod variant has no Python at all, so there is no Python-bearing variant to confine to a devel track. The benchmark/devel container with FastAPI lives as a **sibling** bundle (`ociger-pgck-bench`) that connects to a running ck-allinone over NATS — never as a profile of the same image.

---

## 11. Historical Migration (Pre-Spec → Spec-Aligned → Delta)

Before this spec, all releases landed on the production image name. Two then-published versions were conceptually devel:

- `ociger-ck-allinone:v0.6.0`/`v0.6.6` — bundled FastAPI/pgck-web. **Retired with the Delta migration (2026-05-31).** Latest prod ck-allinone has no Python.
- `ociger-pg17-pgrdf-pgck-web-cklib:*` — **bundle deleted entirely (2026-05-31).** Static-cklib + ck-allinone Delta + sibling ociger-pgck-bench replace its use cases without a Python-bearing prod image.

Historical tags remain on GHCR per the [[only-forward-never-revert]] discipline; LATEST.md no longer advertises them.

---

## 12. Open Questions / Future v1.1+

- **Signed tags**: should production tags be GPG-signed? Devel tags? Not in v1.0; revisit in v1.1.
- **SBOM publishing**: should we attach SBOMs to production-track images automatically (e.g. via `docker buildx` `--sbom` flag)? Not in v1.0.
- **Vulnerability scanning**: gate production promotion on Trivy/Grype scan results? Not in v1.0; manual review for now.
- **Cross-track NOTIFY discipline**: if a devel feature ships and breaks a NOTIFY contract, do we need a NOTIFY-superseded marker? Not in v1.0; per-incident judgement.

---

## 13. References

- `SEMANTIC-VERSIONING.md` — base tagging scheme (2-number tag + git-distance patch).
- `SPEC.OCI.BUNDLE.v0.2.md` — bundle.yaml schema referenced by both tracks.
- `SPEC.NOTIFIES.v0.3` — cross-repo coordination protocol used when notifying about either track (authored in CK.Lib.Js; ask the CK.Lib.Js maintainer for current location).
- `.github/workflows/build-bundles.yml` — CI implementation point for §4.
- `README.md` — public-facing surface; receives §7 pinning guidance after this spec lands.
