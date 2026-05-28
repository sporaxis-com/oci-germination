---
in-reply-to: NOTIFIES.pgRDF.0.5.1.shared-preload-required.md
from: pgRDF
to: oci-germination
date: 2026-05-27
severity: integration-confirmation
---

# RESPONSE — pgRDF 0.5.1 / `shared_preload_libraries` Required

Acknowledging the inbound notify. The diagnosis is correct, the fix is correct, and the requirement is **intended behavior** in pgRDF — not a bug.

## Confirmation

pgRDF's shared-memory atomics (dictionary cache + plan-cache stats) are registered via `pg_shmem_init!`, which pgrx requires to run inside the postmaster's preload context. Loading the `.so` lazily from a backend never enters that context, so the `_PG_init()` guard (`process_shared_preload_libraries_in_progress`) stays false and the atomics are never registered.

Authoritative references in the pgRDF source tree:

| File:Line | What it enforces |
|---|---|
| `src/lib.rs:42-55` | `_PG_init()` checks `process_shared_preload_libraries_in_progress`; only then calls `storage::shmem_cache::init_in_postmaster()` and `query::plan_cache::init_in_postmaster()` |
| `src/lib.rs:36-40` | Doc comment names `shared_preload_libraries` as "the supported production deployment" |
| `src/lib.rs:88-94` | pgRDF's own pg_test harness sets `shared_preload_libraries='pgrdf'` — even the test suite cannot exercise the shmem path without preload |
| `src/storage/shmem_cache.rs:151-165` | `SHMEM_READY` atomic; flipped true only after `init_in_postmaster()`. Every hot path checks `is_ready()` first |

The downstream fix `OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf,pgck` is aligned with this contract.

## Ordering Note

Order in `shared_preload_libraries` matters insofar as `_PG_init()` runs in the listed order. If pgCK's init touches pgRDF shmem during its own startup, `pgrdf,pgck` is the safe order. Reverse order (`pgck,pgrdf`) would still load both but would race the registration. Agreed with the rationale in the notify.

## Verified Downstream Bundles

The fix shipped as `oci-germination` release `v0.1.2` across four bundles. Each was pulled from GHCR and re-smoked against the new `assert_pgrdf_pgatomic` regression block; all return `pgrdf.parse_turtle.pgatomic_ok=1`:

| Image | Tag | Digest |
|---|---|---|
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf` | `v0.1.2` | `sha256:657c0fc2…` |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck` | `v0.1.2` | `sha256:09ca1dcd…` |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats` | `v0.1.2` | `sha256:9da4affd…` |
| `ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro` | `v0.1.2` | `sha256:864fe403…` |

(Multi-arch manifests: `linux/amd64` + `linux/arm64`.)

The original notify's `v0.3` / `v0.4.0` references for `ck-allinone` are still valid as the user-facing ck-allinone numbering; the underlying base-image release is now versioned `v0.1.2` under the per-bundle scheme.

## Defensive Gap Identified in pgRDF (to be closed)

While analysing the notify, an internal inconsistency surfaced. The shmem-cache module guards every `PgAtomic.get()` with `is_ready()` — a lazy-loaded backend degrades gracefully. The plan-cache module does not:

| File:Line | Call | Guarded? |
|---|---|---|
| `src/query/plan_cache.rs:72` | `INSERTS.get().fetch_add(1, …)` in `insert()` | ❌ |
| `src/query/plan_cache.rs:81` | `HITS.get().fetch_add(1, …)` in `record_hit()` | ❌ |
| `src/query/plan_cache.rs:85` | `MISSES.get().fetch_add(1, …)` in `record_miss()` | ❌ |
| `src/query/plan_cache.rs:123-141` | `snapshot()` reads under `is_ready()` | ✅ |

The panic that produced the original `PgAtomic was not initialized` symptom actually fires through `flush_batch()` → `plan_cache::insert(QUAD_INSERT_SQL, plan)` → `INSERTS.get()`. With preload set, this is moot. Without preload, the first `parse_turtle()` panics rather than emitting the more diagnostic "shmem not ready" path the dict cache would take.

**Planned in pgRDF 0.5.2** (no behaviour change in correct deployments):

1. Add `if !shmem_cache::is_ready() { return; }` guards to `plan_cache::{insert, record_hit, record_miss}`. Defense-in-depth only.
2. README section "Required postgresql.conf changes" covering the preload requirement, with the rationale and a copy-paste verification query.

## Two Tripwires Now Active on the Downstream Side

For completeness, the `oci-germination` v0.1.2 release added two regression guards that together prevent this class of failure from reaching GHCR again. pgRDF endorses both as the correct shape:

1. **Static** — `scripts/lint-pgrdf-preload.sh`: every Dockerfile that COPYs `pgrdf.so` must declare `OCIGER_SHARED_PRELOAD_LIBRARIES` with `pgrdf` in the value. Wired into all four pgRDF release workflows.
2. **Runtime** — `scripts/lib/assert-pgrdf-pgatomic.sh`: every pgRDF-bearing smoke script calls `pgrdf.parse_turtle()` and fails fast on the `PgAtomic was not initialized` string.

Both were negative-tested by temporarily reverting the fix; each tripwire fired with a clear remediation hint.

## Action Items

| Owner | Item | Status |
|---|---|---|
| pgRDF | Add `is_ready()` guards to `plan_cache::{insert, record_hit, record_miss}` | planned (0.5.2) |
| pgRDF | Add "Required postgresql.conf changes" section to README | planned (0.5.2) |
| pgRDF | Acknowledge inbound notify | done (this document) |
| oci-germination | Apply same template fix to `bundle-ck-allinone` / `bundle-pg17-pgrdf-pgck-web-cklib` when they next roll forward | tracked downstream |

## Reference

- Inbound: `_WIP/NOTIFIES.pgRDF.0.5.1.shared-preload-required.md`
- pgRDF release: `release/v0.5.1`
- Downstream release tags: `pg17-pgrdf-v0.1.2`, `pg17-pgrdf-pgck-v0.1.2`, `pg17-pgrdf-pgck-nats-v0.1.2`, `pg17-pgrdf-pgck-nats-micro-v0.1.2`
- PostgreSQL `shared_preload_libraries`: https://www.postgresql.org/docs/current/runtime-config-client.html#GUC-SHARED-PRELOAD-LIBRARIES
- Repo: https://github.com/styk-tv/pgRDF
