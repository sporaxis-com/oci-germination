#!/usr/bin/env bash
# lib.sh — helpers for oci-germination's three-state local suite.
#
# Protocol adopted from pgCK tests/v312-tdd (their lib.sh is the reference):
#   exit 0  = GREEN            — the assertion holds on the image under test
#   exit 44 = RED as predicted — the assertion fails in EXACTLY the way the
#             case predicts for an image that predates the pending re-cut.
#             A RED is a suite PASS: it is the build queue, stated as a test.
#   exit 3  = BROKEN           — anything else. A GREEN that fails, a RED that
#             fails differently, or a RED that passes without the ledger
#             moving. The runner fails iff any case is BROKEN.
#
# Cases run against ONE disposable container the runner boots from
# OG_TDD_IMAGE (bind-mounted PGDATA — the deployment shape that burned
# v0.7.31..v0.7.34, not the container-fs shape that hid it). The runner
# exports OG_TDD_CONTAINER / OG_TDD_NETWORK; expectations come from
# versions.yaml via scripts/lib/versions.sh — the single source of truth.
set -u

_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../scripts/lib/versions.sh
source "$_ROOT/scripts/lib/versions.sh"

C="${OG_TDD_CONTAINER:-og-tdd}"
NET="${OG_TDD_NETWORK:-og-tdd-net}"

# Q "sql"  — superuser; QP "sql" — participant (the only externally-scram'd role)
Q()  { docker run --rm --network "$NET" -e PGPASSWORD=ogtdd postgres:18-trixie \
         psql -h "$C" -U postgres -d postgres -At -v ON_ERROR_STOP=0 -c "$1" 2>&1 | tr -d '\r'; }
QP() { docker run --rm --network "$NET" -e PGPASSWORD=ogtdd-part postgres:18-trixie \
         psql -h "$C" -U ck_participant -d postgres -At -v ON_ERROR_STOP=0 -c "$1" 2>&1 | tr -d '\r'; }
# INIMG "cmd" — run inside the image under test (busybox is a single static
# binary with NO applet symlinks: always call /bin/busybox <applet> explicitly,
# a bare `cat`/`sha256sum` silently yields nothing — measured 2026-08-20).
INIMG() { docker exec "$C" /bin/sh -c "$1" 2>&1; }

GREEN()  { echo "GREEN: $1";  exit 0;  }
RED()    { echo "RED (as predicted): $1"; exit 44; }
BROKEN() { echo "BROKEN: $1"; exit 3;  }
