#!/usr/bin/env bash
# GREEN always: the seal plane (ckp._project) is a member of the served set
# (pgck.kernels), every member canonical, and the kernel answers a governed
# read. No RED branch — a divergence here is the v0.7.33 burn, never a queue.
source "$(dirname "$0")/../lib.sh"

SEAL="$(Q "SELECT ckp._project();")"
WIRE="$(Q "SHOW pgck.kernels;")"
[ -n "$SEAL" ] || BROKEN "ckp._project() empty — core-only store; the conf never reached postgres (see 03)"
[ -n "$WIRE" ] || BROKEN "pgck.kernels empty — the wire admits nothing"
member=""
IFS=',' read -ra SET <<< "$WIRE"
for k in "${SET[@]}"; do
  [[ "$k" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]] || BROKEN "served kernel '$k' non-canonical"
  [ "$k" = "$SEAL" ] && member="$k"
done
[ -n "$member" ] || BROKEN "seal plane '$SEAL' not in served set [$WIRE]"
OK="$(QP "SELECT ckp.dispatch('fleet.adoptions','{}'::jsonb)->>'ok';")"
[ "$OK" = "true" ] || BROKEN "agreement vacuous: '$SEAL' does not answer a governed read (ok=$OK)"
GREEN "seals land in '$SEAL', served set [$WIRE], governed read answers"
