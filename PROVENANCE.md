# Build provenance & release policy

## Hard rules

1. **All builds and all GHCR pushes run on GitHub Actions only.** Workstation `docker buildx --push`, `docker push`, `gh release create`, or any equivalent local-credential publish is prohibited at every tier. (The pre-2026-05-28 ck-allinone / web-cklib / static-cklib local pushes were the last; the pipeline migration on that date closed this gap.)
2. **`LATEST.md` MUST NOT carry any version that was published manually or that lacks a verifiable SLSA Build Provenance v1 attestation.** If `gh attestation verify` rejects (or has no record of) the digest in question, that digest is not "the latest" — the file stays where it was. There is no manual-edit exception, not even to seed initial state. When no attested release has been produced yet for a given bundle, `LATEST.md` says so plainly.
3. **The only allowed write to `LATEST.md` is from `.github/workflows/update-latest-md.yml`** (not yet implemented; see Bootstrap below), which renders the file only after `gh attestation verify` accepts every digest it is about to advertise. Any other write is treated as drift and will be reverted by the next workflow run.
4. **A new version tag MUST NOT be pushed unless the previous tag of the same bundle is already advertised in `LATEST.md`.** Concretely: do not tag `release-ck-allinone-v0.6.4` until `v0.6.3` shows up in `LATEST.md`. This guarantees the previous release went through the attestation gate end-to-end. Tagging ahead of the gate breaks the chain and creates orphan releases that the policy cannot retroactively verify.
5. **Release often, in small groups of 1–3 closed tasks per bundle.** Single-task releases are explicitly fine. Larger groupings only when tasks are inherently coupled (a base image bump and its upstream version update, a Dockerfile change and its smoke-script assertion, etc.).
6. **Report version + bundle + closed-tasks every release turn.** When a tag is pushed (or proposed), the user-facing turn summary MUST state:
   - **This turn:** bundle name + semver, plus the task IDs closed (e.g. `ck-allinone:v0.6.3 — Task #10, Task #24`)
   - **Bundles updated:** which of the 11 surfaces were rebuilt this turn
   - **Cross-bundle ripple:** if a base image (`core-pg17-*` or `pg17-pgrdf-pgck-*`) was bumped, list which downstream bundles also need to roll forward
7. **Verify upstream attestation BEFORE doing any work that depends on an upstream artifact.** Before consuming `pgrdf`, `pgck`, `ck-lib-js`, or any other external OCI image in a `Dockerfile`, `bundle.yaml`, smoke script, or release note, run `gh attestation verify oci://<image>:<tag>` against the digest you intend to pin. A 404 or rejection means the artifact is NOT consumable under this policy. The work pauses; the user is notified; we do not proceed until either (a) the upstream ships a valid attestation for that exact digest, or (b) the user explicitly overrides for that specific bump with the override recorded in the commit message and in `LATEST.md` notes.
8. **Voice inconsistencies between upstream `LATEST.md` and actual attestation state immediately, do not accept them.** If an upstream repo's `LATEST.md` advertises a version (e.g. pgRDF's `LATEST.md` listing `0.5.9-pg17-*` as the head) but `gh attestation verify` returns 404 or rejects that digest, that is a verifiable inconsistency. The agent surfaces it in the same turn the discrepancy is found, refuses to consume that version, and recommends one of: bug-report NOTIFY to the upstream, hold-back to the last attested version of theirs, or user-driven explicit override. Silent acceptance of an unattested upstream artifact is prohibited.
9. **Version numbers are monotonic and never reused.** A failed CI run at `vN.M.K` retires that number permanently. The next attempt is `vN.M.(K+1)`, never a re-push of `vN.M.K`. Do not `git push origin :refs/tags/<name>` to delete a failed tag; do not `git tag -f` to move one. The tag stays where the failure happened; the fix is a new commit with a new tag. Gaps in the version sequence are expected — they are explained by the corresponding FAILED entries in `CHANGELOG.md`.
10. **`CHANGELOG.md` records every release attempt, successful or failed; git tags exist only for successful attempts.** Failed attempts (CI failure, attestation failure, smoke failure, etc.) get a FAILED entry in `CHANGELOG.md` describing what was tried and why it failed. They do **not** get a git tag. The version number is permanently spent. `CHANGELOG.md` is the audit trail for gaps in the tag sequence; `LATEST.md` is the audit trail for what's live in production.
11. **`LATEST.md` carries a TEST-CONFIRMED version composition per bundle, not just intended pins.** Each advertised bundle block ends with a *Version composition* table whose **Expected** column is the layer map read from that bundle's `bundle.yaml` (base image, postgres, every extension incl. its native `.version()`, and every runtime/web component with its mapping — `cklib`→`/app/cklib`, busybox httpd→`:8000`, the dispatch relay, …) and whose **Confirmed (real)** column is read back from the *running, attested* image. A native self-report that disagrees with its installed `extversion` is shown as `⚠ stale` with a footnote to its open NOTIFY — surfaced, never silently reconciled. Expected and confirmed must agree (or be a tracked, footnoted upstream deviation) for the entry to stand.
12. **Public-repo disclosure discipline — coordinate in the open, carefully.** This repository and its issues, pull requests, commit messages, and committed docs are **PUBLIC**. Cross-repo coordination (fleet-internal or to upstreams) MAY use public GitHub issues/PRs — **only for content that is safe to disclose**. It MUST NOT expose: internal/confidential component codenames, the contents or paths of gitignored `_WIP/` drafts, security-sensitive internals (key material, unmitigated attack surface, embargoed fixes), or private roadmap / customer / partner detail. Confidential coordination stays in the gitignored `_WIP/` NOTIFIES. When unsure, keep it in `_WIP/` and use neutral language publicly. A leak is a **confidentiality incident**, not a style nit.

Everything else in this document explains how those rules are enforced.

### Public-repo disclosure (Rule 12 — two channels by sensitivity)

- **Public** — GitHub issues / PRs / commits / committed docs. For work that is safe in the open: version bumps, build/CI fixes, the bundle composition, adoption tracking between public fleet repos. Use neutral language; reference upstreams by their public names only.
- **Private** — gitignored `_WIP/` NOTIFIES. For anything confidential: internal consumer codenames, security internals, embargoed upstream fixes, unreleased architecture, partner detail. `_WIP/` is never committed (`.gitignore`); **only `_WIP/` may name confidential entities.**

Before opening or commenting on a public issue/PR (or writing a commit message), ask: *would this sentence be safe on the project's front page?* If not, move it to `_WIP/` and link nothing from the public side. This rule exists because the leak already happened once: an internal consumer codename had reached a tracked smoke comment and several upstream-repo files; it was scrubbed (HEAD-clean, history left per the forward-only rule). Treat the boundary as load-bearing.

### Test-confirmed composition (Rule 11 — how it's produced)

The goal: `LATEST.md` should answer "what is *actually inside* the attested digest", not merely "what we asked the Dockerfile for". Mechanism:

- **Single source for the layer map: `bundle.yaml`.** `tools/version_composition.py` reads `extensions:` + `components:` + `image:` and renders every layer with its version and mapping. No second list to drift.
- **Confirmed live, where the container already runs.** The renderer (`tools/render-latest-md.py`, the only allowed `LATEST.md` writer — Rule 3) probes the *pushed, attested* image and reads back each native surface (`SHOW server_version`, `pg_extension.extversion`, `pgrdf.version()`, `pgck_version()`). The result is written to `bundles/<dir>/composition.confirmed.json`, **stamped with the index digest it was confirmed against**.
- **Digest-gated, never stale.** A `composition.confirmed.json` decorates a `LATEST.md` block **only if its recorded `index_digest` equals the digest being advertised**. A confirmation from a prior cut is ignored; the block falls back to the expected layer map until the next gated build re-confirms. Re-probe is best-effort (`PROBE_COMPOSITION=1`): a flaky probe never breaks the render.
- **Non-probeable layers are gate-confirmed.** Layers without a queryable self-report (busybox, s6, NATS, cklib, the in-tree Go binaries) show `✓ gated` — their presence/behaviour is asserted by the bundle's gate-before-push smoke (busybox serves `/cklib`, NATS answers core+WSS, the relay round-trips), which is a precondition of the digest existing at all.

Example (`ociger-ck-allinone` @ `v0.7.21`):

| Layer | Kind | Expected | Mapping | Confirmed (real) | Verdict |
|-------|------|----------|---------|------------------|---------|
| `postgresql` | engine | `17` | `server` | `17.10 (Debian)` | ✓ |
| `pgrdf` | extension | `0.6.6` | `CREATE EXTENSION` | `0.6.6` | ✓ |
| `pgrdf.version()` | native | `0.6.6` | `self-report` | `0.6.6` | ✓ |
| `pgck` | extension | `0.4.14` | `CREATE EXTENSION` | `0.4.14` | ✓ |
| `pgck.version()` | native | `0.4.14` | `self-report` | `pgck 0.4.3 (rc3)` | ⚠ stale¹ |
| `cklib` | component | `1.5.2` | `/app/cklib` | gate-before-push | ✓ gated |

¹ `pgck_version()` is a frozen literal in the pgCK `.so` (the extension build itself is the correct 0.4.14 — see `extversion`); tracked in the open pgCK `pgck_version()`-stale NOTIFY, not release-blocking. pgRDF derives its `.version()` correctly.

### Release attempt policy (Rules 9, 10 — the changelog ↔ tag separation)

The three files have distinct jobs, and writers must not cross them:

| File | What it captures | Write trigger |
|---|---|---|
| `CHANGELOG.md` | **Every** release attempt for the attestable bundles — SHIPPED entries with digest + CI run ID, and FAILED entries with cause + CI run ID. The honest narrative of how each version got to GHCR or didn't. | Hand-authored on the same commit that tags a successful release, OR on the commit that lands the fix for a previously-failed attempt. |
| Git tags | **Only** successful attempts. One tag per successful release. Failed attempts are never tagged. | `git tag <name>; git push origin <name>` only after the SLSA attestation gate passes locally, OR after CI green for the same content. |
| `LATEST.md` | **Only** the latest attested digest per bundle. Auto-rendered. | `.github/workflows/update-latest-md.yml` only — refuses to advertise any digest that `gh attestation verify` rejects. No manual edit. |

A failure flow looks like this:

```
attempt v0.1.K → tag v0.1.K → CI fails → FAILED entry in CHANGELOG.md → fix commit on main →
attempt v0.1.(K+1) → tag v0.1.(K+1) → CI green → attestation verified →
SHIPPED entry in CHANGELOG.md → LATEST.md auto-regen advances
```

The tag `v0.1.K` is never moved. The fix commit gets `v0.1.(K+1)`. The CI history shows `v0.1.K ref FAILED` followed by `v0.1.(K+1) ref SUCCESS` — unambiguous.

### Bootstrap status (2026-06-11)

The attestation pipeline and `update-latest-md.yml` auto-regen are **landed and in steady-state operation**. As of 2026-06-11:

- `actions/attest-build-provenance@v1` runs on every release workflow.
- `update-latest-md.yml` is the only writer of `LATEST.md`; it runs on `workflow_run` of every successful release build and gates each entry on `gh attestation verify`.
- Both `ociger-ck-allinone` and `ociger-pg17-pgrdf-pgck-nats-micro` have crossed their per-bundle bootstrap. Rule 4 (predecessor must be in `LATEST.md`) holds strictly for them.
- The other 9 matrix bundles (`ociger-core-pg17-*`, `ociger-pg17-pgrdf{,-pgck{,-nats,-static-cklib,-web-cklib}}`, `ociger-pgck-bench`) crossed their bootstrap on the 2026-05-28 .. 2026-05-31 wave; their last attested digests are reflected in `LATEST.md`. Future cuts on those bundles inherit the full Rule 1–10 contract.

The earlier "Current pre-policy LATEST.md state" warning is retired; everything currently advertised in `LATEST.md` is attested.

---

Every artifact this repo publishes — all 11 OCI bundles — is built and pushed **exclusively** by GitHub Actions. Workstation pushes are not permitted at any tier.

## What's enforced

| Surface | Build / push performed by | Provenance |
|---|---|---|
| `ghcr.io/sporaxis-com/ociger-ck-allinone:<ver>` | `Build OCI Bundles` workflow on `release-ck-allinone-v*` tag push | Pre-policy (no attestation yet); will be [SLSA Build Provenance v1](https://slsa.dev/spec/v1.0/provenance) via [`actions/attest-build-provenance@v1`](https://github.com/actions/attest-build-provenance) after the pipeline lands |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib:<ver>` | same workflow on `release-pg17-pgrdf-pgck-static-cklib-v*` | same |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:<ver>` | `pg17-pgrdf-pgck-nats-micro-release.yml` on every `main` push | same |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:<ver>` | `pg17-pgrdf-pgck-nats-release.yml` | same |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:<ver>` | `pg17-pgrdf-pgck-release.yml` | same |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf:<ver>` | `pg17-pgrdf-release.yml` | same |
| `ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:<ver>` | `core-pg17-nats-micro-release.yml` | same |
| `ghcr.io/sporaxis-com/ociger-core-pg17-nats:<ver>` | `core-pg17-nats-release.yml` | same |
| `ghcr.io/sporaxis-com/ociger-core-pg17-micro:<ver>` | `core-pg17-micro-release.yml` | same |
| `ghcr.io/sporaxis-com/ociger-core-pg17-min:<ver>` | `core-pg17-release.yml` | same |
| `LATEST.md` at the repo root | (pending) `update-latest-md` workflow on successful `workflow_run` of the above | Refuses to advance unless `gh attestation verify` accepts every digest it's about to publish |

If `gh attestation verify` rejects an artifact (post-bootstrap), `LATEST.md` stays where it was. That's how a workstation push gets caught — it cannot produce a valid GitHub-issued OIDC attestation.

## Verifying a release locally (post-attestation)

```sh
# Headline bundle
gh attestation verify oci://ghcr.io/sporaxis-com/ociger-ck-allinone:v0.6.3 \
  --repo sporaxis-com/oci-germination

# Static-only web variant
gh attestation verify oci://ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib:v0.6.3 \
  --repo sporaxis-com/oci-germination
```

A successful verify means:

- Signed by GitHub's Fulcio CA against the OIDC token of a specific workflow run
- That workflow run is in `sporaxis-com/oci-germination`
- The signature is recorded in Sigstore's Rekor transparency log
- The subject digest matches the artifact you pulled (multi-arch manifest list, both `amd64` and `arm64` per-platform digests included)

## Verifying upstream artifacts BEFORE bumping a pin (Rule 7 in action)

```sh
# pgRDF — before bumping the FROM line in any *-pgrdf-* Dockerfile
gh attestation verify oci://ghcr.io/styk-tv/pgrdf-bundle:0.5.9-pg17-amd64 \
  --repo styk-tv/pgRDF

# pgCK — before bumping the ORAS pull tag
gh attestation verify oci://ghcr.io/styk-tv/pgck:0.1.7-pg17-amd64 \
  --repo styk-tv/pgCK

# CK.Lib.Js — before bumping static_web.source_image
gh attestation verify oci://ghcr.io/conceptkernel/ck-lib-js:1.3.0 \
  --repo ConceptKernel/CK.Lib.Js
```

Any of these returning **404 Not Found** or rejection blocks the bump. Surface it to the user, propose a NOTIFY to the upstream repo, hold back to the last attested version of theirs (or refuse to consume that surface entirely), and do not silently proceed.

**Snapshot of upstream attestation state (2026-05-28):** all three of pgRDF 0.5.9, pgCK 0.1.7, and CK.Lib.Js 1.3.0 return 404. pgRDF's `LATEST.md` advertises `0.5.9` as the head; pgCK's and CK.Lib.Js's `LATEST.md` correctly say "no attested release yet". Under Rule 8, the pgRDF inconsistency is voiced; under Rule 7, we cannot bump any of the three until they ship attestations or you override.

## Cutting a release (the only allowed flow)

1. Bump bundle.yaml + Dockerfile (versions, source images, supervisor profiles, etc.).
2. Commit (`feat:`, `fix:`, `chore:` per Conventional Commits).
3. Tag: `git tag -a release-<bundle>-v<NEW> -m "<short>"` (e.g. `release-ck-allinone-v0.6.3`).
4. Push the tag: `git push origin <tag>`.

GitHub Actions takes over. There is no step in this flow that requires `docker buildx --push`, `docker push`, `gh release create`, or any local-token credential.

### Per-bundle tag format

| Bundle | Tag prefix |
|---|---|
| `ck-allinone` | `release-ck-allinone-v*` |
| `pg17-pgrdf-pgck-static-cklib` | `release-pg17-pgrdf-pgck-static-cklib-v*` |
| `pg17-pgrdf-pgck-nats-micro` | `release-pg17-pgrdf-pgck-nats-micro-v*` |
| `pg17-pgrdf-pgck-nats` | `release-pg17-pgrdf-pgck-nats-v*` |
| `pg17-pgrdf-pgck` | `release-pg17-pgrdf-pgck-v*` |
| `pg17-pgrdf` | `release-pg17-pgrdf-v*` |
| `core-pg17-nats-micro` | `release-core-pg17-nats-micro-v*` |
| `core-pg17-nats` | `release-core-pg17-nats-v*` |
| `core-pg17-micro` | `release-core-pg17-micro-v*` |
| `core-pg17-min` | `release-core-pg17-v*` |

See [`SEMANTIC-VERSIONING.md`](./SEMANTIC-VERSIONING.md) for the 2-number vs 3-number tag form and patch derivation rules.

## Hooks that block accidental local pushes

The repo's `.gitignore` keeps OCI credentials out of the tree. The bundle Dockerfiles do not call out to local registries. If you find yourself reaching for `docker buildx --push` or `docker push`: stop, push the tag instead, and let CI publish.

## Cross-bundle ripple (Rule 6, expanded)

Many bundles compose on top of others published from this same repo:

```
core-pg17-min ──┐
                ├─→ pg17-pgrdf ──→ pg17-pgrdf-pgck ──┬─→ pg17-pgrdf-pgck-nats ──┐
core-pg17-nats ─┘                                    │                            │
                                                     │                            │
                                                     │                            ▼
                                core-pg17-nats-micro ┴─→ pg17-pgrdf-pgck-nats-micro
                                                                  │
                                                                  ├─→ ck-allinone (Delta — s6-overlay + busybox httpd)
                                                                  └─→ pg17-pgrdf-pgck-static-cklib (ociger-static-server)
```

When a base bundle is rebuilt (e.g. `pg17-pgrdf-pgck-nats-micro:v0.1.3`), every downstream bundle that pins it via `FROM` will continue to resolve to the OLD digest until its own `Dockerfile` is bumped to the new tag and a new tag is pushed. The release-turn report (Rule 6) must call out which downstreams are now lagging so the next release sequence picks them up explicitly.

## Audit trail

- Workflow source: `.github/workflows/build-bundles.yml` (web-bearing bundles) and `.github/workflows/<bundle>-release.yml` (infrastructure bundles)
- Lint pipeline: `.github/workflows/lint.yml` (go vet, gofmt, golangci-lint, shellcheck, hadolint)
- Attestation generator (pending): `actions/attest-build-provenance@v1` (Sigstore-backed)
- Verifier (pending): `gh attestation verify` (built into `gh` 2.49+)
- Renderer (pending): `tools/render-latest-md.py`
- Spec: this file ([`PROVENANCE.md`](./PROVENANCE.md)) — the binding contract
- Related: [`SEMANTIC-VERSIONING.md`](./SEMANTIC-VERSIONING.md)

## Implementation status

- [x] Wire `actions/attest-build-provenance@v1` into `.github/workflows/build-bundles.yml` (web bundles)
- [x] Wire same into 8 per-bundle release workflows (`core-pg17-*-release.yml`, `pg17-pgrdf-*-release.yml`)
- [x] Add `.github/workflows/update-latest-md.yml` driven by `workflow_run`
- [x] Add `tools/render-latest-md.py` (query GHCR + verify attestations + render file)
- [x] Reset `LATEST.md` to the "no attested release yet" template
- [ ] First post-attestation tag per bundle = bootstrap, populates that bundle's entry — pending the next release tag push on any bundle

The pipeline gate is armed. The next release tag push (any bundle) will:
1. Build, push, and attest the artifact in the release workflow
2. Trigger `update-latest-md.yml` on workflow success
3. Run `gh attestation verify` against the digest
4. If ✓: render and commit the populated `LATEST.md` block for that bundle
5. If ✗: leave `LATEST.md` showing "no attested release yet" for that bundle

Subsequent tags on other bundles flow through the same gate, populating their own `LATEST.md` blocks one by one.
