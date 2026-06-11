---
title: Semantic Versioning for OCI Bundles
version: 2.0
date: 2026-06-11
supersedes: v1.0 (2026-05-27 — 2-number tag scheme, retired)
---

# Semantic Versioning Scheme

This document defines how container image versions are calculated for OCI bundles in this repository. The current scheme is a strict SemVer with explicit per-bundle tag prefixes and a hard rule that **version numbers are monotonic and never reused** (see [PROVENANCE.md](./PROVENANCE.md) §Hard rules 9 and 10).

## Version Format

**Semantic Version:** `vMAJOR.MINOR.PATCH`

Examples: `v0.7.12` (ck-allinone), `v0.1.11` (pg_base), `v0.6.7` (static-cklib).

The tag string IS the published image tag — no conversion, no `git describe` arithmetic.

## Git Tags (source of truth)

Each bundle has its own tag prefix because the GitHub Actions workflows trigger off matching patterns:

| Bundle | Tag pattern | Example | Resulting image |
|---|---|---|---|
| `ck-allinone` | `release-ck-allinone-v<MAJOR.MINOR.PATCH>` | `release-ck-allinone-v0.7.12` | `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.7.12` |
| `pg17-pgrdf-pgck-static-cklib` | `release-pg17-pgrdf-pgck-static-cklib-v<…>` | `release-pg17-pgrdf-pgck-static-cklib-v0.6.7` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-static-cklib:v0.6.7` |
| `pg17-pgrdf-pgck-nats-micro` (pg_base) | `pg17-pgrdf-pgck-nats-micro-v<…>` (**no `release-` prefix**) | `pg17-pgrdf-pgck-nats-micro-v0.1.11` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.11` |
| `pg17-pgrdf-pgck-nats` | `pg17-pgrdf-pgck-nats-v<…>` | `pg17-pgrdf-pgck-nats-v0.1.7` | matching GHCR tag |
| `pg17-pgrdf-pgck` | `pg17-pgrdf-pgck-v<…>` | same shape | matching GHCR tag |
| `pg17-pgrdf` | `pg17-pgrdf-v<…>` | same shape | matching GHCR tag |
| `core-pg17-{nats,nats-micro,micro,min}` | `core-pg17-<…>-v<…>` | `core-pg17-nats-micro-v0.1.2` | matching GHCR tag |
| `pgck-bench` | `pgck-bench-v<…>` | `pgck-bench-v0.1.1` | matching GHCR tag |

**Rule:** tag prefix must match the workflow trigger or the workflow won't fire. The pg_base family deliberately does NOT use the `release-` prefix because that prefix is consumed by `build-bundles.yml`, which builds layered bundles on top of pg_base.

## How CI uses the tag

1. The tag push triggers the matching workflow.
2. The workflow extracts the `v<MAJOR.MINOR.PATCH>` suffix and uses it directly as the published image tag.
3. The same string is written into the OCI manifest as `org.opencontainers.image.version`.
4. SLSA Build Provenance v1 attestation is issued binding the digest to the tag.
5. `update-latest-md.yml` runs on `workflow_run` completion, verifies the attestation, and writes the new head into `LATEST.md`.

No `git describe` distance arithmetic. No 2-number → 3-number conversion. The tag string IS the version, end to end.

## Bumping the version

| Change shape | Bump |
|---|---|
| Compositional pin change (upstream extension, library, base image) | PATCH |
| New in-tree feature (new launcher env, new smoke gate, new relay generation) | PATCH (alpha track) or MINOR (once a stable surface is declared) |
| Breaking surface change (Dockerfile arg removed, env semantics inverted, init.sql restructured) | MINOR (alpha track) or MAJOR (post-1.0) |

These bundles are on the `0.x.y` alpha track. The MAJOR=0 stays until the CKP v3.x runtime contract stabilises; until then, breaking changes ride MINOR bumps, with the breakage documented in `CHANGELOG.md`.

## Monotonic + never-reused (the hard rule)

Once `vN.M.K` has a tag, that number is **permanently spent**, regardless of outcome:

- **SHIPPED** — the CI run completed, the image is on GHCR, the attestation verifies. Next bump is `v(N).(M).(K+1)`.
- **FAILED** — the CI run failed; no artifact reached GHCR. The version is **still spent**. Next bump is `v(N).(M).(K+1)`, NOT a re-push of `vN.M.K`. The failure is recorded in [CHANGELOG.md](./CHANGELOG.md) with the failing step + cause.

Operationally: do not run `git push origin :refs/tags/<name>`. Do not run `git tag -f <name>`. The fix is a new commit with the next version number.

## Cross-bundle pin discipline

When a base bundle moves (`pg_base` is the common case), downstream bundles that consume it as a FROM-image must roll forward to consume the new tag. Today only `ck-allinone` consumes `pg_base` directly; the others in the family are sibling-composed.

A ripple is reported in the release turn's user-facing summary (per PROVENANCE.md rule 6).

## Cross-references

- [CHANGELOG.md](./CHANGELOG.md) — every release **attempt**, SHIPPED and FAILED, with verdict + cause.
- [LATEST.md](./LATEST.md) — auto-rendered head of each bundle, attestation-gated.
- [PROVENANCE.md](./PROVENANCE.md) — the full release policy (Rules 1–10).
- [CONTRIBUTING.CI.md](./CONTRIBUTING.CI.md) — the day-to-day "how do I cut a release" walkthrough.
