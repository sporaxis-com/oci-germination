---
title: Bundle Generator Implementation Roadmap
version: 1.0
date: 2026-05-27
status: Planned
---

# Bundle Generator Implementation Roadmap

## Current Status

- ✅ **SPEC.OCI.BUNDLE.v0.2** published with `static_web` field definition
- ✅ **bundle.yaml files** updated with `spec_version: 0.2` and `static_web` declarations
- ✅ **Dockerfiles** manually implement the expected output from static_web declarations
- ⏳ **Dockerfile generator** to automate the generation (planned, not yet implemented)

## What Needs to Be Generated

The bundle generator should read `bundle.yaml` and auto-generate:

### 1. Build Stages for Each Static Web Mount

Input:
```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
  - source_image: ghcr.io/org/docs:v1
    route: /docs
```

Generated Dockerfile stages:
```dockerfile
FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source
FROM ghcr.io/org/docs:v1 AS docs_source
```

### 2. COPY Commands in Builder Stage

For each static_web entry, copy from its source stage:
```dockerfile
FROM python:3.11-slim AS builder
COPY --from=cklib_source / /app/cklib/
COPY --from=docs_source / /app/docs/
RUN mkdir -p /app/cklib /app/docs && ...
```

### 3. FastAPI Mount Configuration

Auto-generate the mount logic in main.py:
```python
from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from pathlib import Path

app = FastAPI(title="pgck-web", version="0.1.0")

# Auto-generated from static_web:
cklib_dir = Path(__file__).parent / "cklib"
if cklib_dir.exists():
    app.mount("/cklib", StaticFiles(directory=str(cklib_dir)), name="cklib")

docs_dir = Path(__file__).parent / "docs"
if docs_dir.exists():
    app.mount("/docs", StaticFiles(directory=str(docs_dir)), name="docs")
```

## Implementation Plan

### Phase 1: Validation (Immediate)
- [ ] Create Python script to validate bundle.yaml against v0.2 schema
- [ ] Verify all bundles have valid `static_web` declarations
- [ ] Test validation against example bundles

### Phase 2: Generator (Next Sprint)
- [ ] Implement Dockerfile template engine
- [ ] Parse `static_web` from bundle.yaml
- [ ] Generate complete Dockerfile with all stages
- [ ] Generate FastAPI app with dynamic mounts
- [ ] Output to bundle directory overwriting previous Dockerfile

### Phase 3: CI/CD Integration
- [ ] Add generator step to bundle build workflow
- [ ] Verify generated Dockerfile matches bundle.yaml
- [ ] Fail build if hand-edited Dockerfile diverges from generated version
- [ ] Document in CONTRIBUTING.md: "Never hand-edit bundle Dockerfiles"

### Phase 4: Cleanup (Optional)
- [ ] Remove any existing hand-edited Dockerfile content
- [ ] Verify all bundles use generated Dockerfiles
- [ ] Archive old generation approach

## Current Workaround

Until the generator is implemented:

1. **Define static_web in bundle.yaml** (done ✓)
2. **Manually create Dockerfile** matching the expected generated output
3. **Add comment to Dockerfile**: `# Generated from bundle.yaml per SPEC.OCI.BUNDLE.v0.2`
4. **Keep bundle.yaml as source of truth** for route/image changes

### Updating Routes

To change a mount path:
```yaml
# In bundle.yaml, change:
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /client-lib  # Changed from /cklib
```

Then **manually update Dockerfile** to match (until generator is implemented).

## Code Locations

- **bundle.yaml spec**: `SPEC.OCI.BUNDLE.v0.2.md` (section 2)
- **Bundle examples**: `bundles/bundle-ck-allinone/bundle.yaml`, `bundles/bundle-pg17-pgrdf-pgck-web-cklib/bundle.yaml`
- **Generator script** (to be created): `tool/bundle-generator.py` (proposed)

## Acceptance Criteria

The generator is "done" when:
- ✅ Reads all fields from bundle.yaml (image, extensions, components, static_web, services, platforms)
- ✅ Generates multi-stage Dockerfile with proper build flow
- ✅ Generates FastAPI app with dynamic static file mounts
- ✅ Produces Dockerfiles that build successfully
- ✅ Output matches current hand-edited Dockerfiles in behavior
- ✅ Can be run as: `python tool/bundle-generator.py <bundle-dir>`
- ✅ Enforces validation per SPEC.OCI.BUNDLE.v0.2 §7
