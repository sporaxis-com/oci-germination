#!/bin/sh
# CHECKLIST.NEXT-CK-ALLINONE §A — first-boot auto-bootstrap.
#
# Runs as an s6-rc oneshot AFTER postgres is up. Idempotent: marker file
# in $PGDATA makes re-runs a no-op (survives container restart on a
# persistent volume).
#
# What it does on a fresh volume:
#   • waits for postgres to accept connections
#   • CREATE EXTENSION pgcrypto + pgrdf + pgck CASCADE (in bootstrap DB 'postgres')
#   • touches $PGDATA/.ck-allinone.bootstrapped marker
#
# pgck.nats_url is NOT set here — it's already in postgresql.conf via
# OCIGER_POSTGRES_CONF_EXTRA (set on the image's ENV), because pgCK's
# bgworker only reads the GUC once on first tick. Setting it after-the-fact
# would not wake the relay.

set -eu

PGDATA="${PGDATA:-/var/lib/postgresql/data}"
MARKER="${PGDATA}/.ck-allinone.bootstrapped"
PSQL="/usr/local/bin/psql -h 127.0.0.1 -U postgres -d postgres -At -v ON_ERROR_STOP=1"

if [ -f "${MARKER}" ]; then
  echo "bootstrap-ckp: marker present, skipping (idempotent re-boot)"
  exit 0
fi

# Wait up to 60 s for postgres to accept connections.
echo "bootstrap-ckp: waiting for postgres to accept connections..."
i=0
while ! ${PSQL} -c 'SELECT 1' >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 60 ]; then
    echo "bootstrap-ckp: timed out waiting for postgres" >&2
    exit 1
  fi
  /bin/busybox sleep 1
done
echo "bootstrap-ckp: postgres ready (after ${i}s)"

# Run the bootstrap SQL. IF NOT EXISTS makes this a no-op when the user
# created extensions manually on an existing volume.
${PSQL} <<'SQL'
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pgrdf;
CREATE EXTENSION IF NOT EXISTS pgck CASCADE;
SQL

/bin/busybox touch "${MARKER}"
echo "bootstrap-ckp: ✓ pgcrypto + pgrdf + pgck installed; marker written"
