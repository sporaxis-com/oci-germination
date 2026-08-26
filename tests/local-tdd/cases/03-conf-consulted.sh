#!/usr/bin/env bash
# GREEN always: the boot-provisioned pgck.conf is ACTUALLY read by postgres
# (source = configuration file, never the compiled default). No RED branch —
# a silent include-skip is the v0.7.31..v0.7.34 identity outage, never a queue.
source "$(dirname "$0")/../lib.sh"

SRC="$(Q "SELECT source FROM pg_settings WHERE name='pgck.nats_url';" | tr -d ' ')"
if [ "$SRC" != "configurationfile" ]; then
  INIMG "/bin/busybox ls -ln /run/ck-identity/pgck.conf"
  BROKEN "pgck.conf NOT consulted (pgck.nats_url source='$SRC') — every pgck.* GUC is silently defaulting"
fi
GREEN "pgck.conf consulted (source=configuration file)"
