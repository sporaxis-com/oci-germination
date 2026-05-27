---
notifies: CK.Lib.Js
version: 1.2.0
theme: oci-bundle-source-layer
from: oci-germination
date: 2026-05-27
severity: integration-confirmation
---

# NOTIFY CK.Lib.Js 1.2.0 — Verified as OCI Bundle Source Layer

## Context

`oci-germination` (Sporaxis-Com OCI bundles) consumes `ghcr.io/conceptkernel/ck-lib-js:1.2.0` as a **declarative static web source layer** per `SPEC.OCI.BUNDLE.v0.2`.

## Confirmed Compliance (CK.Lib.Js v1.2.0)

- ✅ Image URI: `ghcr.io/conceptkernel/ck-lib-js:1.2.0`
- ✅ Designation: `ckp:static` (Dockerfile label)
- ✅ Multi-platform: `linux/amd64`, `linux/arm64`
- ✅ No hand-edited Dockerfile (matches "Dockerfiles are generated" principle)
- ✅ Mount-friendly size (~2MB)

## Consumption Pattern

Downstream `bundle.yaml` declares:

```yaml
spec_version: 0.2
static_web:
  - source_image: ghcr.io/conceptkernel/ck-lib-js:1.2.0
    route: /cklib
```

Generator auto-creates Dockerfile stages:

```dockerfile
FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source
COPY --from=cklib_source / /app/cklib/
```

FastAPI auto-mounts at the declared route:

```python
app.mount("/cklib", StaticFiles(directory="/app/cklib"), name="cklib")
```

## Active Consumers (as of 2026-05-27)

| Bundle | Image | Mount Route |
|---|---|---|
| `pg17-pgrdf-pgck-web-cklib` | `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.4.0` | `/cklib` |
| `ck-allinone` | `ghcr.io/sporaxis-com/ociger-ck-allinone:v0.4.0` | `/cklib` |

Both consumers serve CK.Lib.Js files at `http://<container>:8000/cklib/`.

## Action Items for CK.Lib.Js

**None required.** This notification is for awareness only. CK.Lib.Js 1.2.0 is a properly-formed source layer per the v0.2 spec.

**Future requests** (non-blocking):

- If CK.Lib.Js bumps to 1.3.0+, downstream bundles will need to update `source_image` in their `bundle.yaml`.
- Breaking changes to the file layout under `/` (e.g. moving `ck-client.js` to a subdirectory) would require route adjustments downstream.

## Reference

- Spec: `SPEC.OCI.BUNDLE.v0.2.md` (oci-germination)
- Consuming bundles: `bundles/bundle-pg17-pgrdf-pgck-web-cklib/bundle.yaml`, `bundles/bundle-ck-allinone/bundle.yaml`
- Repo: https://github.com/sporaxis-com/oci-germination
