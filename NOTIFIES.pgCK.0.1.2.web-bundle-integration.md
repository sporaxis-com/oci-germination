---
notifies: pgCK
version: 0.1.2
theme: web-bundle-integration
from: oci-germination
date: 2026-05-27
severity: integration-confirmation
---

# NOTIFY pgCK 0.1.2 — Web Bundle Integration & shared_preload Co-dependency

## Context

`oci-germination` bundles pgCK 0.1.2 alongside pgRDF 0.5.1, FastAPI (pgckweb), and optionally NATS. Two findings worth recording upstream:

## 1. Shared Preload Co-dependency with pgRDF

pgCK MUST be preloaded after pgRDF in `shared_preload_libraries`:

```dockerfile
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck
```

**Reason:** pgCK extension functions appear to reference pgRDF symbols during `_PG_init()`. Loading pgCK without pgRDF preloaded causes downstream `ckp.boot()` failures.

See also: `NOTIFIES.pgRDF.0.5.1.shared-preload-required.md` (sibling notification).

## 2. pgckweb (FastAPI) Mount Pattern

`oci-germination` bundles pgckweb (FastAPI app from `styk-tv/pgCK/tree/main/web` and `web_demo`) into OCI images:

- **Standard variant** (`pg17-pgrdf-pgck-web-cklib`): pgckweb 0.2.0 from `web_demo`
- **All-in-one** (`ck-allinone`): pgckweb 0.1.0 from `web`

Both mount FastAPI at port `8000` and serve CK.Lib.Js at `/cklib/`.

## 3. Verified Functional Containers

```
ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.4.0
  - PostgreSQL 17 + pgRDF 0.5.1 + pgCK 0.1.2 + pgckweb 0.2.0 + cklib 1.2.0

ghcr.io/sporaxis-com/ociger-ck-allinone:v0.4.0
  - Above + NATS 2.14.1 core (4222) + WSS bridge (9222) + supervisor
```

## Action Items for pgCK

**Recommended documentation update** (non-blocking):

1. Document that `shared_preload_libraries` MUST include both `pgrdf` and `pgck` (in that order) for proper initialization.
2. Consider noting in the pgCK README that `web/` and `web_demo/` subdirectories are valid pgckweb entry points for OCI bundlers.

## Pending Upstream Items

- `pgckweb` versioning: currently informally tracked. Formal semver tags in `styk-tv/pgCK` for the `web/` subtree would help downstream bundlers pin precisely.

## Reference

- Resolution doc: `ISSUE.BASE-IMAGE.RESOLVED.md`
- Bundle specs: `bundles/bundle-pg17-pgrdf-pgck-web-cklib/bundle.yaml`, `bundles/bundle-ck-allinone/bundle.yaml`
- Repo: https://github.com/sporaxis-com/oci-germination
