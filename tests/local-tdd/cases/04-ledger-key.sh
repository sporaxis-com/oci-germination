#!/usr/bin/env bash
# GREEN always (0.4.80+): the ledger identity key is a minted 32-byte value,
# never the pre-0.4.80 shared literal, and ckp.config is dump-flagged so the
# key travels with the facts it signs.
source "$(dirname "$0")/../lib.sh"

K="$(Q "SELECT v FROM ckp.config WHERE k='identity_key';" | tr -d ' ')"
[ -n "$K" ] || BROKEN "no identity_key on a fresh install — sealing will refuse"
[ "$K" != "pgck-localhost" ] || BROKEN "identity_key is the shared literal — proof chains attribute nothing"
[[ "$K" =~ ^[0-9a-f]{64}$ ]] || BROKEN "identity_key not 32 bytes hex (${#K} chars)"
D="$(Q "SELECT (SELECT count(*)>0 FROM pg_extension e, unnest(e.extconfig) c WHERE e.extname='pgck' AND c='ckp.config'::regclass)::text;" | tr -d ' ')"
[ "$D" = "true" ] || BROKEN "ckp.config not dump-flagged — a restore loses the ledger's key"
GREEN "ledger key minted (${K:0:12}…) and dump-flagged"
