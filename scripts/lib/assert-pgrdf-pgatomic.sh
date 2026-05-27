#!/usr/bin/env bash
# Regression guard for the pgRDF PgAtomic-initialization failure.
#
# pgRDF's _PG_init() registers shared-memory atomics via pg_shmem_init!,
# which pgrx requires to run in the postmaster's preload context. When
# `pgrdf` is missing from `shared_preload_libraries` the .so loads lazily
# in a backend, _PG_init runs WITHOUT process_shared_preload_libraries_in_progress,
# the atomics are never registered, and the first call to pgrdf.parse_turtle
# panics with: "PgAtomic was not initialized".
#
# Sourcing this file installs:
#   assert_pgrdf_pgatomic <db>
#
# The caller must already define a `psql_query <db> <sql>` shell function
# (every pgRDF-bearing smoke script in scripts/smoke-pg17-*.sh does).

assert_pgrdf_pgatomic() {
  local db="${1:?db required}"
  local sql="SELECT pgrdf.parse_turtle('PREFIX ex: <http://example.org/> ex:test a ex:Thing .', 1::bigint, 'http://example.org/');"
  local output rc=0
  output="$(psql_query "$db" "$sql" 2>&1)" || rc=$?

  if printf '%s' "$output" | grep -q 'PgAtomic was not initialized'; then
    {
      echo "REGRESSION: pgRDF PgAtomic not initialized."
      echo "  Cause: pgrdf missing from shared_preload_libraries when postmaster started."
      echo "  Fix:   ENV OCIGER_SHARED_PRELOAD_LIBRARIES must contain 'pgrdf' (before 'pgck' if both)."
      echo "  Captured output:"
      printf '%s\n' "$output"
    } >&2
    return 1
  fi

  if (( rc != 0 )); then
    {
      echo "pgrdf.parse_turtle returned an unexpected error (rc=$rc):"
      printf '%s\n' "$output"
    } >&2
    return "$rc"
  fi

  echo "pgrdf.parse_turtle.pgatomic_ok=1"
}
