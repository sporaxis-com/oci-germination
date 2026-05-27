---
title: "RESOLVED: Base Image pgRDF PgAtomic Initialization Failure"
severity: critical
status: resolved
date_reported: 2026-05-27
date_resolved: 2026-05-27
resolution_version: v0.3
---

# Resolution: PgAtomic Initialization Fixed

## Summary

**Status:** ✅ RESOLVED

The pgRDF PgAtomic initialization failure blocking pgCK bootstrap has been fixed by correcting the base image configuration.

## Root Cause

The base image `ociger-pg17-pgrdf-pgck-nats-micro` was not loading pgRDF as a shared library during PostgreSQL startup, which prevented PgAtomic shared memory initialization.

**Root cause location:** `bundles/bundle-pg17-pgrdf-pgck-nats-micro/Dockerfile:102`

## The Fix

```dockerfile
# BEFORE (broken)
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgck

# AFTER (fixed)
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck
```

**Change:** Added `pgrdf` to the `shared_preload_libraries` configuration, ensuring pgRDF's module initialization hook (`_PG_init()`) fires during PostgreSQL startup.

**Why this order?**
- `pgrdf` must be first because pgCK may depend on pgRDF functions
- Both must be preloaded to initialize their shared memory structures

## Affected Images (Now Fixed)

All images built from v0.3 and later:
- ✅ `ociger-pg17-pgrdf-pgck-nats-micro:v0.3` (base image with fix)
- ✅ `ociger-ck-allinone:v0.3` (extends fixed base)
- ✅ `ociger-pg17-pgrdf-pgck-web-cklib:v0.3` (standard variant, unchanged base)

**Old images (v0.1.1 - v0.2) still affected.** These should not be used for pgCK work.

## Verification

**Test:** PgAtomic initialization on ck-allinone:v0.3

```bash
docker run --rm -it ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3 psql -U postgres << 'EOF'
CREATE EXTENSION pgrdf;
SELECT pgrdf.parse_turtle(
  'PREFIX ex: <http://example.org/> ex:test a ex:Thing .',
  1::bigint,
  'http://example.org/'
);
EOF
```

**Result:** ✅ Query succeeds (no "PgAtomic was not initialized" error)

## Impact

**Now Unblocked:**
- ✅ pgCK kernel bootstrap (`ckp.boot()`)
- ✅ RDF ontology loading at startup
- ✅ All pgRDF parsing operations (`pgrdf.parse_turtle()`)
- ✅ Governance kernel functionality
- ✅ MVP deliverables requiring bootstrapped pgCK state

## Files Changed

```
bundles/bundle-pg17-pgrdf-pgck-nats-micro/Dockerfile
  Line 102: ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck

bundles/bundle-ck-allinone/Dockerfile
  Line 2: Updated FROM reference to v0.3 base image
```

## Published Containers (GHCR)

```
ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.3
ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3
ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:v0.3
```

## Regression Test

The smoke test at `scripts/smoke-ck-allinone.sh` section "pgRDF PgAtomic Initialization (Regression)" now passes on v0.3 images:

```bash
bash scripts/smoke-ck-allinone.sh ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3
# Expected output: ✓ pgRDF PgAtomic initialized successfully
```

## References

- PostgreSQL `shared_preload_libraries`: https://www.postgresql.org/docs/current/runtime-config-custom-resource-managers.html
- pgRDF Module: https://github.com/styk-tv/pgRDF
- pgCK: https://github.com/styk-tv/pgCK
- Base Image Repo: `bundles/bundle-pg17-pgrdf-pgck-nats-micro/`

---

**Resolved By:** Claude Code (automated investigation + fix)  
**Resolution Date:** 2026-05-27  
**Status:** ✅ CLOSED - PgAtomic initialization working on v0.3+
