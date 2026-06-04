# SPEC.INDEX.v1.0 — Authoritative catalogue of repository specifications

**Version:** 1.0
**Status:** Authoritative. This is the public landing index for every specification this repository governs.
**Date:** 2026-06-04
**Audience:** Downstream consumers of the bundles published from this repository, and anyone wishing to follow the contracts this repository declares.

---

## §0. Purpose

This repository publishes OCI bundles to GHCR under the `ghcr.io/sporaxis-com/ociger-*` namespace. The specifications governing those bundles have grown across several versioned documents; this index is the canonical map between them. A reader who lands here SHALL be able to identify, in one read, which spec is current, which has been superseded, and which external specifications this repository depends on.

The index itself is small and stable. The detailed specifications are each their own document.

---

## §1. How to use this index

```
┌────────────────────────────────────────────────────────────────────────────────────────────────┐
│  You are                                              Start at                                │
├───────────────────────────────────────────────────────┼───────────────────────────────────────┤
│  ▸ Consuming an `ociger-*` image from GHCR            │  §3 — bundle contract                 │
│  ▸ Reading the `org.opencontainers.image.*` manifest  │  §3                                   │
│    labels to understand what the image is             │                                       │
│  ▸ Trying to compose a bundle (write a Dockerfile,    │  §3 + §4                              │
│    s6 services, smoke script)                         │                                       │
│  ▸ Wondering why a bundle's component bears a         │  §4 — devel-vs-prod tracks            │
│    `ck.bundle.never-prod=true` label                  │                                       │
│  ▸ Following the cross-repository NOTIFY protocol     │  §6 — external references             │
│  ▸ Following the CKP v3.8 ontology that pgCK uses     │  §6                                   │
│  ▸ Verifying a bundle's SLSA attestation              │  §5 — attestation                     │
└───────────────────────────────────────────────────────┴───────────────────────────────────────┘
```

---

## §2. Repository scope

This repository, `https://github.com/sporaxis-com/oci-germination`, produces:

```
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│  Family             │  GHCR namespace                                  │  Published          │
├─────────────────────┼──────────────────────────────────────────────────┼─────────────────────┤
│  ck-allinone        │  ghcr.io/sporaxis-com/ociger-ck-allinone         │  prod               │
│  pg-base (micro)    │  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-    │  prod               │
│                     │      nats-micro                                   │                     │
│  pgrdf-only family  │  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-…        │  prod               │
│  static-cklib       │  ghcr.io/sporaxis-com/ociger-static-cklib        │  prod               │
│  web-cklib family   │  ghcr.io/sporaxis-com/ociger-web-…               │  prod               │
└─────────────────────┴──────────────────────────────────────────────────┴─────────────────────┘
```

The repository **never compiles upstream code**. It assembles bundles by layering on top of published upstream images and adding in-tree Go binaries, supervisor service trees, and static assets. Any apparent need to patch upstream code is resolved via the NOTIFIES protocol (§6) — never by an in-tree patch.

---

## §3. Bundle composition contract — authoritative

### Current

```
┌─────────────────────────────────────────────┬──────────────────────────────────────────────────┐
│  SPEC.OCI.BUNDLE.v0.4.md                    │  Authoritative bundle contract.                  │
│                                              │  Defines: bundle.yaml shape, the role /          │
│                                              │  never-prod manifest labels, the Python-free     │
│                                              │  CI gate, the Delta composition rule, the        │
│                                              │  scratch-base requirement for production         │
│                                              │  images, the no-compile rule, the no-cross-      │
│                                              │  component-patching rule, and the public         │
│                                              │  attestation chain. Read this before composing   │
│                                              │  any new bundle.                                  │
└─────────────────────────────────────────────┴──────────────────────────────────────────────────┘
```

### Superseded (kept for archaeology)

```
┌─────────────────────────────────────────────┬──────────────────────────────────────────────────┐
│  SPEC.OCI.BUNDLE.v0.3.md                    │  Superseded by v0.4. Last spec to allow Python  │
│                                              │  inside production bundles; the v0.4 ratchet     │
│                                              │  introduced the Python-free CI gate and Delta    │
│                                              │  composition.                                     │
│  SPEC.OCI.BUNDLE.v0.2.md                    │  Superseded by v0.3. Introduced the explicit     │
│                                              │  bundle.yaml renderer and the static-cklib       │
│                                              │  variant.                                         │
│  SPEC.OCI.BUNDLE.v0.1.md                    │  Superseded by v0.2. Original sporaxis-com      │
│                                              │  compatible OCI image definition. Retained for   │
│                                              │  historical reference.                            │
└─────────────────────────────────────────────┴──────────────────────────────────────────────────┘
```

### Bundle composition lifecycle (forward-looking, no commitment)

The composition rule set in `SPEC.OCI.BUNDLE.v0.4.md` is the surface every public bundle SHALL satisfy. Internal work on a declarative composition model — typed entities and typed predicates rendered into the existing Dockerfile + s6 + smoke artifacts by a deterministic composer — is in progress at workspace-confidential scope. Adoption of any internal model SHALL be accompanied by a public spec version bump (presumptive next: `SPEC.OCI.BUNDLE.v0.5.md`) before any change to the bundle.yaml contract this index points at. No bundle in this repository currently relies on the internal work; existing consumers are unaffected.

---

## §4. Track distinction — devel vs prod

```
┌─────────────────────────────────────────────┬──────────────────────────────────────────────────┐
│  SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md         │  Authoritative.                                  │
│                                              │  Defines the difference between the devel        │
│                                              │  surface (transitional / migration / debug       │
│                                              │  bundles) and the prod surface. Governs the      │
│                                              │  semantics of the `ck.bundle.never-prod=true`    │
│                                              │  manifest label and which bundle families are    │
│                                              │  ineligible for promotion.                       │
└─────────────────────────────────────────────┴──────────────────────────────────────────────────┘
```

Every bundle in this repository SHALL carry both `ck.bundle.role` (`prod` or `devel`) and `ck.bundle.never-prod` (`true` or `false`) manifest labels per this spec.

---

## §5. Attestation contract

Every published bundle SHALL carry SLSA Build Provenance v1 attestation produced by the repository's GitHub Actions workflows. Verification SHALL use `gh attestation verify` against the bundle's GHCR digest.

There is currently no separate attestation specification; the contract is "exactly what `actions/attest-build-provenance@v1` emits, gated by the per-bundle smoke test passing." Smoke pass is therefore a prerequisite for attestation, by construction of the workflow. A future spec MAY freeze this binding explicitly; for now it is enforced procedurally in `.github/workflows/build-bundles.yml` and family-specific release workflows.

---

## §6. External references

This repository depends on the following specifications maintained elsewhere. They are referenced here so consumers and contributors can find them; this repository does not modify them.

```
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│  CKP v3.8 ontology — `https://conceptkernel.org/ontology/v3.8/`                              │
│      The ck-allinone bundle ships the pgCK extension; pgCK loads this ontology into its      │
│      runtime graph. The bundle's bootstrap SQL issues `CREATE EXTENSION pgck CASCADE`,       │
│      which triggers ckp.boot() and the ontology load. The runtime contract is owned by       │
│      the pgCK project; this repository is only a delivery vehicle.                           │
│                                                                                              │
│  CKP v3.8 predicate discipline — `SPEC.CKP.v3.8-rc-08-predicates.md` (pgCK repo)             │
│      Governs which predicates have runtime meaning in pgCK vs which are descriptive only.    │
│      This repository's composition vocabulary (when published) SHALL declare its rc-08       │
│      classification explicitly and SHALL NOT subProperty onto any runtime-critical pgCK      │
│      predicate. Treated as binding for any cross-namespace work.                              │
│                                                                                              │
│  NOTIFIES protocol — `SPEC.NOTIFIES.v0.3` (CK.Lib.Js repo)                                   │
│      Cross-repository coordination protocol used when this repository needs upstream         │
│      action it cannot or shall not perform locally. Examples: asking pgRDF to ship a new     │
│      SONAME-stable release; asking pgCK to publish a build with the nats-client feature.    │
│      This repository SHALL file a NOTIFY rather than patch the upstream.                     │
│                                                                                              │
│  pgRDF — `https://github.com/styk-tv/pgRDF`                                                  │
│      Upstream owner of the pgRDF extension shipped in the pg17-pgrdf-* bundle family and    │
│      inherited transitively by ck-allinone. The pg_base image's Debian release tracks        │
│      pgRDF's CI compile target.                                                              │
│                                                                                              │
│  pgCK — `https://github.com/styk-tv/pgCK`                                                    │
│      Upstream owner of the pgCK extension shipped in the same family. Build features         │
│      affect what code is actually present in the published .so; consumers who need a         │
│      specific feature SHALL request it via NOTIFY.                                            │
└──────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## §7. Manifest label contract — what an image declares about itself

Every image published from this repository SHALL carry the following manifest labels. The labels are part of the public contract; consumers MAY rely on them.

```
┌──────────────────────────────────────────┬──────────────────────────────────────────────────┐
│  org.opencontainers.image.source         │  The GitHub repository URL.                      │
│  org.opencontainers.image.description    │  One-line human description of the bundle.       │
│  org.opencontainers.image.licenses       │  SPDX identifier (typically `MIT`).              │
│  org.opencontainers.image.version        │  The bundle's release version, e.g. `v0.7.6`.    │
│                                          │  This SHALL match the GHCR tag, not the version  │
│                                          │  of any parent image inherited via `FROM`.       │
│  ck.bundle.role                          │  `prod` | `devel`. Per `SPEC.OCIGERMI.TRACKS`.   │
│  ck.bundle.never-prod                    │  `true` | `false`. Per the same.                 │
│  ck.spec.index                           │  URL of THIS document (this very specification). │
│                                          │  Forward-stable pointer for consumers wanting to │
│                                          │  start their reading here.                        │
└──────────────────────────────────────────┴──────────────────────────────────────────────────┘
```

The `org.opencontainers.image.version` rule is binding because Docker's `LABEL` inherits transitively from `FROM`; a bundle that fails to emit its own explicit `LABEL org.opencontainers.image.version` will silently advertise the parent image's version. Every bundle's Dockerfile final stage SHALL emit this label explicitly.

---

## §8. Versioning of this index

This index is `v1.0`. It is intended to be stable across multiple bundle releases. The following kinds of change require a version bump:

```
┌────────────────────────────────────────────────────┬─────────────────────────────────────────┐
│  A bundle spec is promoted, superseded, or         │  v1.1                                   │
│  retired (table in §3 changes)                     │                                         │
│  A new external reference becomes load-bearing     │  v1.1                                   │
│  (table in §6 changes)                             │                                         │
│  The manifest label contract gains or drops a      │  v1.1                                   │
│  required label (table in §7 changes)              │                                         │
│  A new specification family appears in the         │  v2.0                                   │
│  repository (e.g. SPEC.OCI.COMPOSITION.*)          │                                         │
└────────────────────────────────────────────────────┴─────────────────────────────────────────┘
```

The version of THIS index is what `ck.spec.index` URLs SHALL pin to in practice; the URL embeds the version, so a `v1.1` will live at a distinct filename and consumers reading an older image label will retrieve the index they were promised.

---

## §9. Quick reference — current pinned state

At the time this index was issued (2026-06-04):

```
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│  Bundle (release)                                Notes                                       │
├──────────────────────────────────────────────────────────────────────────────────────────────┤
│  ck-allinone v0.7.6        (this release wave)   prod, ready-to-use, ~125 MiB, scratch base │
│  pg-base v0.1.7            (parent of above)     prod, inherited by ck-allinone              │
│  static-cklib v0.6.6                             prod                                        │
│  pgrdf-* family v0.1.7                           prod                                        │
│  web-cklib family v0.1.5                         prod                                        │
└──────────────────────────────────────────────────────────────────────────────────────────────┘
```

The authoritative current list lives in `LATEST.md` at the repository root, refreshed automatically after each successful attestation. This index points at families; `LATEST.md` points at exact tags.

---

## §10. Where to read next

- `SPEC.OCI.BUNDLE.v0.4.md` — bundle composition contract, current.
- `SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md` — devel/prod track semantics.
- `LATEST.md` — the live tag advertisement.
- `bundles/<bundle-name>/bundle.yaml` — per-bundle declared composition.
- `https://github.com/sporaxis-com/oci-germination/releases` — release notes for the cataloged bundles.
