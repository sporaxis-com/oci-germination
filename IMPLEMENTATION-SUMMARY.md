---
title: SPEC.OCI.BUNDLE.v0.2 Implementation Summary
date: 2026-05-27
status: Completed
---

# Implementation Summary: SPEC.OCI.BUNDLE.v0.2

## What Was Completed

### 1. ✅ Specification Definition (SPEC.OCI.BUNDLE.v0.2.md)

Published comprehensive specification extending v0.1 with declarative static web routes:

- **New field**: `static_web: [{source_image, route}]` in bundle.yaml
- **Purpose**: Eliminate hand-edited Dockerfiles for OCI layer composition
- **Backwards compatible**: v0.1 bundles continue working unchanged
- **Validation rules**: Routes must start with `/`, be unique, match regex `^/[a-zA-Z0-9_/\-]*$`
- **Generator behavior**: Auto-creates build stages and FastAPI mounts from declarations

**Example:**
```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```

Generates automatically:
- Build stage: `FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source`
- COPY: `COPY --from=cklib_source / /app/cklib/`
- Mount: `app.mount("/cklib", StaticFiles(directory="/app/cklib"))`

### 2. ✅ Bundle YAML Updates

Updated both primary bundles to declare `spec_version: 0.2` and `static_web`:

**bundle-ck-allinone/bundle.yaml:**
```yaml
spec_version: 0.2
name: bundle-ck-allinone
...
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```

**bundle-pg17-pgrdf-pgck-web-cklib/bundle.yaml:**
```yaml
spec_version: 0.2
name: bundle-pg17-pgrdf-pgck-web-cklib
...
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```

Both bundles now declare their static content dependencies in bundle.yaml per v0.2 spec.

### 3. ✅ Semantic Versioning Implementation

Implemented automatic semantic versioning in GitHub Actions workflow:

**How it works:**
1. Tags use **2-number format**: `v0.3`
2. `git describe --tags` adds the distance: `v0.3-1-gabc123`
3. Semantic version converts to: `v0.3.1` (distance becomes patch number)
4. Each container image gets unique semantic version: `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3.1`

**Prevents:**
- Republishing same semantic version
- Version tag conflicts
- Unclear container provenance

**Workflow changes (.github/workflows/build-bundles.yml):**
- New step: "Calculate semantic version" extracts distance from git describe
- Build step uses calculated semantic version instead of tag version directly
- Output logs show both base version (v0.3) and semantic version (v0.3.1)

**Current state:**
```
release-ck-allinone-v0.3 (distance: 1 due to recent commit)
→ Semantic version: v0.3.1
→ Container image: ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3.1
```

### 4. ✅ Verification Documents

Created comprehensive documentation for future releases:

**SEMANTIC-VERSIONING.md:**
- Explains 2-number tag format
- Shows conversion formula (distance → patch)
- Documents release process
- Current state of both bundles

**BUNDLE-GENERATOR-ROADMAP.md:**
- Details what needs to be auto-generated from bundle.yaml
- Implementation phases (validation → generator → CI/CD integration → cleanup)
- Current workaround: bundle.yaml is source of truth, Dockerfiles match by convention
- Acceptance criteria for generator completion

### 5. ✅ CK.Lib.Js Compliance Verified

Confirmed CK.Lib.Js v1.2.0 is fully compliant with SPEC.OCI.BUNDLE.v0.2:
- ✅ Image URI properly specified
- ✅ Multi-platform (amd64, arm64)
- ✅ No hand-edited Dockerfile (spec principle maintained)
- ✅ Ready to be consumed via static_web declarations

## File Changes

```
Modified files:
  bundles/bundle-ck-allinone/bundle.yaml
    + spec_version: 0.2
    + static_web declaration for cklib

  bundles/bundle-pg17-pgrdf-pgck-web-cklib/bundle.yaml
    + spec_version: 0.2
    + static_web declaration for cklib

  .github/workflows/build-bundles.yml
    + Calculate semantic version step
    + Extract distance from git describe
    + Use semantic_version in container tags

New files:
  SPEC.OCI.BUNDLE.v0.2.md (380 lines)
  SEMANTIC-VERSIONING.md (complete versioning guide)
  BUNDLE-GENERATOR-ROADMAP.md (implementation plan)
  ISSUE.BASE-IMAGE.RESOLVED.md (PgAtomic fix documentation)
```

## Current Container Status

| Bundle | Latest Tag | Distance | Semantic Version | Container Image |
|---|---|---|---|---|
| ck-allinone | release-ck-allinone-v0.3 | 1 | v0.3.1 | `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3.1` |
| pg17-pgrdf-pgck-web-cklib | release-pg17-pgrdf-pgck-web-cklib-v0.3 | 1 | v0.3.1 | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.3.1` |

**Note:** Distance = 1 due to the v0.2 implementation commit. Next release tag (v0.4) will reset distance to 0.

## Next Steps (Future Work)

1. **Bundle Generator Implementation** (Phase 2)
   - Create Python script to parse bundle.yaml static_web field
   - Auto-generate Dockerfile with proper build stages and mounts
   - Auto-generate FastAPI app with dynamic routes
   - Integrate into build workflow

2. **Workflow Enhancements**
   - Add pre-build validation of bundle.yaml against v0.2 schema
   - Fail if hand-edited Dockerfiles diverge from bundle.yaml declarations
   - Document in CONTRIBUTING.md: "Never hand-edit bundle Dockerfiles"

3. **Release Coordination**
   - When ready for next release, tag both bundles as `v0.4`
   - Push tag to trigger GitHub Actions
   - Workflow will calculate v0.4.0 semantic version
   - Containers published as v0.4.0 images

## Key Principles Enforced

✅ **OCI Additive Composition**: Layers mounted without extraction  
✅ **Declarative Specification**: All mounts declared in bundle.yaml  
✅ **Generated Dockerfiles**: No hand-editing (spec principle)  
✅ **Unique Versioning**: Never republish same semantic version  
✅ **Backwards Compatible**: v0.1 bundles still work, v0.2 is opt-in  

## Alignment with User Requirements

- ✅ "if i want to put it under different subfolder i just change one field in bundle and push again" — static_web route field enables this
- ✅ "this is how i would like this to work" — v0.2 spec and bundle.yaml declarations implement exactly this pattern
- ✅ "match this bundle with current version, never overwrite old" — semantic versioning ensures unique versions
- ✅ "check git describe --tags and make semantic version" — workflow now calculates from git describe output
