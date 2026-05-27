---
title: "Base Image: pgRDF PgAtomic Initialization Failure (shared_preload_libraries)"
severity: critical
status: open
date_reported: 2026-05-27
repository: ociger-pg17-pgrdf-pgck-nats-micro (base image)
affected_bundles:
  - ociger-ck-allinone:v0.2
  - ociger-pg17-pgrdf-pgck-web-cklib:v0.2
---

# Base Image Issue: pgRDF PgAtomic Not Initialized

## Problem Summary

The base image `ociger-pg17-pgrdf-pgck-nats-micro:v0.1.1` is missing `pgrdf` in PostgreSQL's `shared_preload_libraries` configuration. This prevents pgRDF's module initialization hook from running during PostgreSQL startup, which means the PgAtomic shared memory structure is never initialized.

**Result:** Any call to pgRDF functions (e.g., `pgrdf.parse_turtle()`) fails with:
```
ERROR:  PgAtomic was not initialized
```

This blocks:
- `ckp.boot()` (blocks pgCK initialization)
- All pgRDF RDF parsing operations
- Any governance kernel bootstrap that uses RDF ontologies

## Root Cause

PostgreSQL modules that use shared memory (like pgRDF's PgAtomic) require being loaded during server startup via `shared_preload_libraries`. Currently:

```sql
shared_preload_libraries = 'pgck'
```

Should be:

```sql
shared_preload_libraries = 'pgrdf,pgck'
```

When a module is NOT preloaded, its `_PG_init()` hook never fires, so internal structures (like PgAtomic) are never initialized.

## Affected Images

- `ociger-pg17-pgrdf-pgck-nats-micro:v0.1.1` ← **ROOT CAUSE**
- All images that extend this base:
  - `ociger-pg17-pgrdf-pgck-nats:v0.1.1`
  - `ociger-ck-allinone:v0.2` (oci-germination bundle)
  - `ociger-pg17-pgrdf-pgck-web-cklib:v0.2` (oci-germination bundle)

## Reproduction

### Step 1: Start container from base image
```bash
docker run -d --name pgrdf-test \
  -p 5432:5432 \
  ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.1
sleep 5
```

### Step 2: Create database and load extensions
```bash
psql -h 127.0.0.1 -p 5432 -U postgres << 'EOF'
CREATE DATABASE test;
\c test
CREATE EXTENSION pgcrypto;
CREATE EXTENSION pgrdf CASCADE;
CREATE EXTENSION pgck CASCADE;
EOF
```

### Step 3: Attempt any pgRDF function
```bash
psql -h 127.0.0.1 -p 5432 -U postgres -d test << 'EOF'
SELECT pgrdf.parse_turtle(
  'PREFIX ex: <http://example.org/> ex:test a ex:Thing .',
  1::bigint,
  'http://example.org/'
);
EOF
```

### Expected Error
```
ERROR:  PgAtomic was not initialized
```

## Fix Required

### File to Modify
Locate where `shared_preload_libraries` is set in the base image build (likely in `postgresql.conf` template or initdb script).

### Change Needed
```diff
- shared_preload_libraries = 'pgck'
+ shared_preload_libraries = 'pgrdf,pgck'
```

### Why This Order?
- `pgrdf` must be first because pgCK may depend on pgRDF functions
- Both must be preloaded to initialize their shared memory structures

### Build Steps After Fix
1. Rebuild base image with updated configuration
2. Tag as `ociger-pg17-pgrdf-pgck-nats-micro:v0.1.2` (patch bump)
3. Push to GHCR
4. All downstream bundles (oci-germination) will automatically pick up the fix on next rebuild with updated base ref

## Verification

After fix is applied and base image rebuilt, test with:

```bash
docker run -it ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.1.2 \
  psql -U postgres << 'EOF'
CREATE EXTENSION pgrdf CASCADE;
SELECT pgrdf.parse_turtle(
  'PREFIX ex: <http://example.org/> ex:test a ex:Thing .',
  1::bigint,
  'http://example.org/'
);
EOF
```

Expected result: Query succeeds and returns RDF data structure (not an error).

## Impact

**Blocks:**
- pgCK kernel bootstrap (ckp.boot fails)
- RDF ontology loading at startup
- Any governance kernel functionality
- MVP deliverables that require bootstrapped pgCK state

**Workaround:** None. Requires base image fix.

## References

- PostgreSQL `shared_preload_libraries`: https://www.postgresql.org/docs/current/runtime-config-custom-resource-managers.html
- pgRDF Module: https://github.com/styk-tv/pgRDF
- Base Image Repo: (location of ociger-pg17-pgrdf-pgck-nats-micro)
- Related Bundle Issue: oci-germination#issue (PgAtomic initialization failure in bundles)

## Detection in CI

The regression test in `oci-germination/scripts/smoke-ck-allinone.sh` (section "pgRDF PgAtomic Initialization Regression") will catch this issue:

```bash
SELECT pgrdf.parse_turtle(...) # Will fail if pgRDF not preloaded
```

Fix base image → rebuild bundles → regression test passes ✓

---

**Created:** 2026-05-27  
**Status:** OPEN - Awaiting base image fix  
**Next Step:** Apply fix to base image, rebuild, tag v0.1.2
