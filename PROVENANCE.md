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

Everything else in this document explains how those rules are enforced.

### One-time bootstrap (Rules 2, 3, 4 transition)

These rules take effect from the **first attested release** onward, per bundle. Releases that predate the attestation wiring (everything published as of 2026-05-28) live in GHCR and on the [Repo packages view](https://github.com/orgs/sporaxis-com/packages?repo_name=oci-germination) but do **not** appear in `LATEST.md` once the attestation gate ships. Re-publishing them with attestations would change their digests and break the immutability promise on GHCR.

For each bundle, the next tag pushed AFTER the `actions/attest-build-provenance@v1` integration lands in `.github/workflows/build-bundles.yml` is the bootstrap point. That workflow run will issue an attestation for the first time, `update-latest-md.yml` will verify and populate `LATEST.md`, and from that point Rule 4 holds strictly for every successor of that bundle.

Bootstrap exception is one-time **per bundle**. Once the gate has fired once for `ociger-ck-allinone`, "previous tag must be in `LATEST.md`" is strict for ck-allinone — even if other bundles haven't crossed their own bootstrap yet.

**Current pre-policy LATEST.md state (2026-05-28):** the current `LATEST.md` was hand-rendered ahead of the attestation pipeline and ships pre-policy versions. That file will be torn down to the "no attested release yet" template on the same commit that lands the attestation workflow, and rebuilt entry-by-entry as each bundle crosses its bootstrap point.

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
- Related specs: [`SEMANTIC-VERSIONING.md`](./SEMANTIC-VERSIONING.md), [`SPEC.OCI.BUNDLE.v0.2.md`](./SPEC.OCI.BUNDLE.v0.2.md), [`SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md`](./SPEC.OCIGERMI.TRACKS.DEVEL.v1.0.md)

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
