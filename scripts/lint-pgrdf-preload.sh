#!/usr/bin/env bash
# Lint: every Dockerfile that copies pgrdf.so directly MUST set
# OCIGER_SHARED_PRELOAD_LIBRARIES with 'pgrdf' in the value, otherwise
# the postmaster will start without preload, _PG_init runs in backend
# context, the PgAtomic shmem is never initialised, and the first
# pgrdf.parse_turtle() call panics with "PgAtomic was not initialized".
#
# Bundles that FROM another image inherit the ENV from the base; this
# lint deliberately skips them (the lint can't statically check the
# transitive base — that's what the runtime smoke does).
#
# Run via CI; intended to fail fast before build + smoke.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

violations=0

while IFS= read -r dockerfile; do
  if ! grep -qE '^COPY --from=pgrdf_fetch .*/pgrdf\.so' "$dockerfile"; then
    continue
  fi

  # A copied pgrdf.so MUST be preloaded by some baked-in mechanism. Two are
  # accepted (both make the postmaster load pgrdf before any backend, so
  # _PG_init runs in postmaster context and the PgAtomic shmem is set up):
  #
  #   (A) ENV OCIGER_SHARED_PRELOAD_LIBRARIES=…pgrdf…  — the ociger-launcher
  #       bundles, whose launcher writes shared_preload_libraries from this env.
  #   (B) a baked  shared_preload_libraries … 'pgrdf'  line — the stock-postgres
  #       bundles (e.g. pg17-bookworm-pgrdf) that append it to the conf template
  #       so initdb bakes it in.

  preload_line="$(grep -E '^ENV OCIGER_SHARED_PRELOAD_LIBRARIES=' "$dockerfile" || true)"
  if [[ -n "$preload_line" ]]; then
    value="${preload_line#ENV OCIGER_SHARED_PRELOAD_LIBRARIES=}"
    case ",${value}," in
      *,pgrdf,*) continue ;;   # mechanism A satisfied
      *)
        echo "FAIL: $dockerfile sets OCIGER_SHARED_PRELOAD_LIBRARIES='$value' which does not include 'pgrdf'" >&2
        echo "      Required: 'pgrdf' must appear in the comma-separated list (before 'pgck' if both present)" >&2
        violations=$((violations + 1))
        continue
        ;;
    esac
  fi

  # mechanism B: a baked shared_preload_libraries config line that includes pgrdf.
  if grep -qE "shared_preload_libraries[^#]*pgrdf" "$dockerfile"; then
    continue
  fi

  echo "FAIL: $dockerfile copies pgrdf.so but preloads it by neither mechanism" >&2
  echo "      Add  ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf  (launcher bundles)" >&2
  echo "      or   a baked  shared_preload_libraries = 'pgrdf'  line (stock-postgres bundles)." >&2
  violations=$((violations + 1))
done < <(find bundles -mindepth 2 -maxdepth 2 -name Dockerfile | sort)

if (( violations > 0 )); then
  echo "" >&2
  echo "lint-pgrdf-preload: $violations violation(s)" >&2
  exit 1
fi

echo "lint-pgrdf-preload: OK"
