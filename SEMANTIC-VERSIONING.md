---
title: Semantic Versioning for OCI Bundles
version: 1.0
date: 2026-05-27
---

# Semantic Versioning Scheme

This document defines how container image versions are calculated for OCI bundles in this repository.

## Version Format

**Semantic Version:** `vMAJOR.MINOR.PATCH`

Example: `v0.3.0`, `v0.3.5`, `v1.0.10`

## How Versions Are Calculated

### 1. Git Tags (Source of Truth)

Tags follow a **2-number format**:
- Format: `release-<bundle-name>-vMAJOR.MINOR`
- Examples:
  - `release-ck-allinone-v0.3`
  - `release-pg17-pgrdf-pgck-web-cklib-v0.2`

### 2. `git describe --tags` Conversion

The semantic version is derived from `git describe --tags`:

```
git describe --tags --match "release-ck-allinone-*"
```

This command returns one of two forms:

**At a tag (distance = 0):**
```
release-ck-allinone-v0.3
→ Semantic version: v0.3.0
```

**N commits since tag:**
```
release-ck-allinone-v0.3-10-gabc1234
→ Semantic version: v0.3.10
                     ↑   ↑  ↑
              MAJOR.MINOR.PATCH
              (patch = distance since tag)
```

### 3. CI/CD Automation

The GitHub Actions workflow (`.github/workflows/build-bundles.yml`) automatically:

1. Extracts the base version from the tag name (e.g., `v0.3`)
2. Runs `git describe --tags` to get the distance since tag
3. Calculates the semantic version (e.g., `v0.3.10`)
4. Tags the built container with the semantic version

## Release Process

### Creating a New Release

When ready to release a new version:

```bash
# 1. Tag both bundles (or just the one being released)
git tag release-ck-allinone-v0.4
git tag release-pg17-pgrdf-pgck-web-cklib-v0.4

# 2. Push tags
git push origin release-ck-allinone-v0.4
git push origin release-pg17-pgrdf-pgck-web-cklib-v0.4

# 3. GitHub Actions automatically:
#    - Detects the tag push
#    - Calculates semantic version v0.4.0
#    - Builds and pushes ghcr.io/sporaxis-com/ociger-ck-allinone:v0.4.0
```

### Between Releases

If you push commits after tagging:

```
git tag release-ck-allinone-v0.3
<push 5 new commits>

git describe --tags --match "release-ck-allinone-*"
# Output: release-ck-allinone-v0.3-5-gabc1234
# Semantic version: v0.3.5
```

This allows for patch-level versioning without requiring new tags.

## Current State

| Bundle | Latest Tag | Semantic Version |
|---|---|---|
| `ck-allinone` | `release-ck-allinone-v0.4` | `v0.4.0` |
| `pg17-pgrdf-pgck-web-cklib` | `release-pg17-pgrdf-pgck-web-cklib-v0.4` | `v0.4.0` |

Both are published to GHCR as:
- `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.4.0`
- `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.4.0`

## Key Principles

- **Never republish the same semantic version** — each container image must have a unique version
- **Tags use 2-number format** — the patch number is calculated from git distance
- **CI/CD is automatic** — the workflow handles version calculation; developers just push tags
- **Backwards compatible** — existing images with v0.3 tags remain unchanged and available
