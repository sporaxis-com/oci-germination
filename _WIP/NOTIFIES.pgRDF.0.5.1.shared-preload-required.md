---
notifies: pgRDF
version: 0.5.1
theme: shared-preload-required
from: oci-germination
date: 2026-05-27
severity: critical-context
---

# NOTIFY pgRDF 0.5.1 — `shared_preload_libraries` Required for PgAtomic Initialization

## Context

This notification is from `oci-germination` (Sporaxis-Com OCI bundles) to the pgRDF upstream project. When pgRDF is bundled into a PostgreSQL OCI image, it MUST be listed in `shared_preload_libraries` for `_PG_init()` to fire during PostgreSQL startup. Without it, `PgAtomic` shared memory structures are never initialized, and any call to `pgrdf.parse_turtle()` or other operations using atomic state fails with:

```
ERROR: PgAtomic was not initialized
```

## Where Resolved Downstream

The fix in `oci-germination`:

```dockerfile
# bundles/bundle-pg17-pgrdf-pgck-nats-micro/Dockerfile:102
ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck
```

Order matters: `pgrdf` MUST come before `pgck` because pgCK may depend on pgRDF functions during its own initialization.

## Suggested Upstream Action

Consider documenting this requirement prominently in the pgRDF README / install docs:

- The `pgrdf.control` file could declare module status, but PostgreSQL only invokes `_PG_init()` when:
  1. The extension is listed in `shared_preload_libraries`, OR
  2. The shared library is loaded explicitly via `LOAD 'pgrdf'`
- Without preload, `CREATE EXTENSION pgrdf` succeeds but PgAtomic remains uninitialized.

A README section like "Required postgresql.conf changes" would prevent downstream bundlers from hitting this trap.

## Verified Fix Containers

These containers are confirmed to initialize pgRDF correctly:

```
ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.3
ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.4.0
ghcr.io/sporaxis-com/ociger-ck-allinone:v0.3
ghcr.io/sporaxis-com/ociger-ck-allinone:v0.4.0
```

Regression test (passes on v0.3+):

```bash
docker run --rm -it ghcr.io/sporaxis-com/ociger-ck-allinone:v0.4.0 psql -U postgres << 'EOF'
CREATE EXTENSION pgrdf;
SELECT pgrdf.parse_turtle(
  'PREFIX ex: <http://example.org/> ex:test a ex:Thing .',
  1::bigint,
  'http://example.org/'
);
EOF
```

## Reference

- Resolution doc: `ISSUE.BASE-IMAGE.RESOLVED.md`
- PostgreSQL `shared_preload_libraries`: https://www.postgresql.org/docs/current/runtime-config-client.html#GUC-SHARED-PRELOAD-LIBRARIES
- Repo: https://github.com/sporaxis-com/oci-germination
