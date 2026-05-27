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

  preload_line="$(grep -E '^ENV OCIGER_SHARED_PRELOAD_LIBRARIES=' "$dockerfile" || true)"
  if [[ -z "$preload_line" ]]; then
    echo "FAIL: $dockerfile copies pgrdf.so but has no OCIGER_SHARED_PRELOAD_LIBRARIES env" >&2
    echo "      Add: ENV OCIGER_SHARED_PRELOAD_LIBRARIES=pgrdf  (or 'pgrdf,pgck' if pgck also present)" >&2
    violations=$((violations + 1))
    continue
  fi

  value="${preload_line#ENV OCIGER_SHARED_PRELOAD_LIBRARIES=}"
  case ",${value}," in
    *,pgrdf,*) : ;;
    *)
      echo "FAIL: $dockerfile copies pgrdf.so but OCIGER_SHARED_PRELOAD_LIBRARIES='$value' does not include 'pgrdf'" >&2
      echo "      Required: 'pgrdf' must appear in the comma-separated list (before 'pgck' if both present)" >&2
      violations=$((violations + 1))
      ;;
  esac
done < <(find bundles -mindepth 2 -maxdepth 2 -name Dockerfile | sort)

if (( violations > 0 )); then
  echo "" >&2
  echo "lint-pgrdf-preload: $violations violation(s)" >&2
  exit 1
fi

echo "lint-pgrdf-preload: OK"
