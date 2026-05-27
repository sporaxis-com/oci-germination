---
title: "SPEC.OCI.BUNDLE.v0.2 — Sporaxis-Com OCI Bundle Definition with Static Web Routes"
version: 0.2
date: 2026-05-27
status: Stable
supersedes: SPEC.OCI.BUNDLE.v0.1
---

# SPEC.OCI.BUNDLE.v0.2 — Static Web Routes & Declarative OCI Composition

This specification extends **v0.1** with declarative static web mount support, enabling OCI layers to be composed into FastAPI without hand-edited Dockerfile COPY operations.

**Breaking changes:** None. v0.1 bundles continue to work. v0.2 adds optional `static_web` field.

---

## 1. What's New in v0.2

### 1.1 Static Web Routes (`static_web` field)

Declare OCI layers that should be mounted as static web content in FastAPI, without hand-editing Dockerfile or extracting/copying.

**Pattern:**
```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib                           # Mounted at this Envoy-style path
```

**Generator behavior:**
1. Adds `source_image` as a build stage (e.g., `FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source`)
2. Copies all files from that layer into `/app/<route>` in the builder stage
3. FastAPI automatically mounts `StaticFiles(directory="/app/<route>")` at the specified route
4. Final image includes the static files at the route path

**Result:**
- ✅ Additive OCI composition (layer is included, not extracted-and-copied)
- ✅ Zero hand-editing of Dockerfile
- ✅ Configurable routes: change one field in bundle.yaml → change mount path
- ✅ Follows spec: Dockerfile is generated, not hand-edited

### 1.2 Multiple Static Routes

A bundle can declare multiple static web mounts:

```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
  - source_image: ghcr.io/org/assets:v1
    route: /assets
  - source_image: ghcr.io/org/docs:v2
    route: /docs
```

Each gets its own stage and mount point.

---

## 2. Updated `bundle.yaml` Schema (v0.2)

### 2.1 New Top-Level Field

| Field | Type | Required | Description |
|---|---|---|---|
| `static_web` | array of objects | no | OCI layers to mount as static web routes |

### 2.2 `static_web[]` Object Schema

| Field | Type | Required | Description |
|---|---|---|---|
| `source_image` | string | yes | OCI image URI (with tag or digest). Example: `ghcr.io/conceptkernel/ck-lib-js:1.2.0` |
| `route` | string | yes | URL path for FastAPI mount (Envoy-style, leading `/`). Examples: `/cklib`, `/assets`, `/staticlib` |

**Semantics:**
- `source_image`: Full OCI reference. Use tags or digests to pin versions. The build stage pulls this image.
- `route`: The URL path where FastAPI will serve these files. Must start with `/`. Examples:
  - `/cklib` → `http://container:8000/cklib/`
  - `/assets` → `http://container:8000/assets/`
  - `/staticlib` → `http://container:8000/staticlib/` (if you later change bundle.yaml to this, rebuild)

**Validation:**
- `route` MUST start with `/` and contain no spaces or special characters (alphanumerics, `-`, `_`, `/` only).
- `source_image` MUST be a valid OCI reference.
- Multiple routes MUST be unique (no duplicates).

---

## 3. Dockerfile Generation (v0.2 Updates)

### 3.1 Build Stage for Each Static Web Mount

For each entry in `static_web`, the generator creates a stage:

```dockerfile
FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source
FROM ghcr.io/org/assets:v1 AS assets_source
```

### 3.2 Copying to Builder Stage

In the `builder` stage, copy each static web mount:

```dockerfile
FROM python:3.11-slim AS builder
COPY --from=cklib_source / /app/cklib/
COPY --from=assets_source / /app/assets/
RUN mkdir -p /app/static /app/cklib /app/assets && ...
```

### 3.3 FastAPI Mount Configuration

The FastAPI app generator includes all declared routes:

```python
from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from pathlib import Path

app = FastAPI(title="pgck-web", version="0.1.0")

# Auto-generated from static_web:
cklib_dir = Path(__file__).parent / "cklib"
if cklib_dir.exists():
    app.mount("/cklib", StaticFiles(directory=str(cklib_dir)), name="cklib")

assets_dir = Path(__file__).parent / "assets"
if assets_dir.exists():
    app.mount("/assets", StaticFiles(directory=str(assets_dir)), name="assets")
```

---

## 4. Migration from v0.1 to v0.2

### 4.1 Updating Existing Bundles

If you have a v0.1 bundle with hand-edited Dockerfile that does COPY/mount for static content:

**Before (v0.1, hand-edited Dockerfile):**
```dockerfile
FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source
...
COPY --from=cklib_source / /app/cklib/
RUN mkdir -p /app/cklib && printf '...' > /app/main.py
app.mount("/cklib", ...)
```

**After (v0.2, bundle.yaml + generated Dockerfile):**

```yaml
# bundle.yaml (new field)
name: bundle-ck-allinone
...
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```

Then regenerate:
```bash
bundle render bundles/bundle-ck-allinone
```

The new Dockerfile will include the COPY and mount logic automatically. **Delete hand-edited Dockerfile content** — it's now generated.

### 4.2 Version Declaration

Bundles can declare which spec version they follow:

```yaml
spec_version: 0.2
name: bundle-ck-allinone
...
```

Bundles without this field are assumed to be v0.1 (for backwards compatibility).

---

## 5. Example: Converting `bundle-ck-allinone` to v0.2

### Before (v0.1, current state)

**bundles/bundle-ck-allinone/bundle.yaml:**
```yaml
name: bundle-ck-allinone
image:
  registry: ghcr.io/sporaxis-com/ociger-ck-allinone
  ...
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
```

**bundles/bundle-ck-allinone/Dockerfile:**
```dockerfile
FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source
...
COPY --from=cklib_source / /app/cklib/  # ← hand-edited, violates spec
```

### After (v0.2)

**bundles/bundle-ck-allinone/bundle.yaml:**
```yaml
spec_version: 0.2
name: bundle-ck-allinone
image:
  registry: ghcr.io/sporaxis-com/ociger-ck-allinone
  ...
services:
  nats:
    source_image: nats:2.14.1-scratch
    core_port: 4222
    websocket_port: 9222
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```

**bundles/bundle-ck-allinone/Dockerfile:** (regenerated, COPY/mount logic auto-included)
```dockerfile
# Generated automatically from bundle.yaml
FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source
FROM python:3.11-slim AS builder
COPY --from=cklib_source / /app/cklib/  # ← auto-generated from static_web
# (rest of builder stage, FastAPI mount auto-generated)
```

**Result:**
- One field in bundle.yaml declares the static web mount
- Dockerfile is fully generated (no hand-editing)
- To move cklib to `/assets` or `/staticlib`, just change `route: /assets` and regenerate
- Follows spec: "Dockerfiles are generated, not hand-edited"

---

## 6. Use Cases

### 6.1 Single Static Layer

```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```
Result: `http://container:8000/cklib/` serves ck-lib-js files.

### 6.2 Multiple Layers at Different Routes

```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
  - source_image: ghcr.io/org/docs:v1
    route: /docs
  - source_image: ghcr.io/org/assets:v2
    route: /public
```
Result:
- `http://container:8000/cklib/` → ck-lib-js
- `http://container:8000/docs/` → docs
- `http://container:8000/public/` → assets

### 6.3 Reconfigurable Routes

**Initially:**
```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```

**Later, change to:**
```yaml
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /client-lib    # New path
```

Rebuild:
```bash
bundle render bundles/bundle-ck-allinone
docker buildx build --push -t ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3 bundles/bundle-ck-allinone
```

Result: Same OCI layer, now served at `/client-lib/` instead of `/cklib/`.

---

## 7. Validation Rules (v0.2 Extensions)

The build system MUST enforce:

- [ ] If `static_web` is declared, it MUST be a non-empty array.
- [ ] Each `static_web[]` entry MUST have `source_image` (non-empty string).
- [ ] Each `static_web[]` entry MUST have `route` (non-empty string starting with `/`).
- [ ] All routes MUST be unique (no duplicates).
- [ ] All routes MUST match regex `^/[a-zA-Z0-9_/\-]*$` (alphanumerics, `-`, `_`, `/`, no spaces).
- [ ] All `source_image` references MUST be valid OCI image URIs.

---

## 8. Backwards Compatibility

- **v0.1 bundles continue to work unchanged.** Bundles without `static_web` field are treated as v0.1.
- **v0.2 is additive.** No existing fields are removed or renamed.
- **Generator detects version:** If bundle.yaml declares `spec_version: 0.2` OR has `static_web` field, use v0.2 generator. Otherwise, use v0.1.

---

## 9. References

- **SPEC.OCI.BUNDLE.v0.1.md** — Original specification (unchanged).
- **SPEC.OCI.BUNDLE.v0.2.md** (this document) — Extensions for static web routes.
- **Bundle Rendering Code** — Will be updated to handle `static_web` field and auto-generate FastAPI mounts.

---

**Document End. SPEC.OCI.BUNDLE.v0.2 is now normative for new Sporaxis-Com OCI bundles with static web components.**
