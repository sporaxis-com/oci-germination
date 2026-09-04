#!/usr/bin/env bash
# wire-ephemeral.sh — prove the ARTIFACT over the wire, with no bench, no
# volume, no compose, and nothing left behind.
#
#   bash scripts/wire-ephemeral.sh                                  # :latest
#   bash scripts/wire-ephemeral.sh ghcr.io/.../ociger-ck-allinone:v0.7.43
#   bash scripts/wire-ephemeral.sh ociger-ck-allinone:local-preflight
#
# EVERY RUN IS VIRGIN BY CONSTRUCTION. The container gets no volume at all, so
# PGDATA lives in its own filesystem and dies with it — there is nothing to
# re-virgin because nothing persists. That is the point: if this passes, the
# published artifact plus environment variables is sufficient, with no operator
# step, no ALTER SYSTEM, and no state carried from a previous run.
#
# It mints a THROWAWAY Ed25519 realm (the same recipe verify-callout.sh uses)
# because since v0.7.43 the door ships CLOSED: a container with no realm refuses
# to boot. So this exercises the real path — realm, callout, verified identity —
# rather than the anonymous shortcut. Case 0 covers the shortcut separately.
#
# Ports are ephemeral and bound to 127.0.0.1. Container name is og- prefixed and
# removed on exit, including on failure.
set -u

IMAGE="${1:-ghcr.io/sporaxis-com/ociger-ck-allinone:latest}"
NAME="og-wire-eph-$$"
NET="og-wire-eph-net-$$"
KERNEL="${KERNEL:-wiretest}"
ISS="${ISS:-https://wire-ephemeral.test/realm}"
AUD="${AUD:-wire-ephemeral}"
SUB="${SUB:-bot-wire-ephemeral}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d)"

pass=0; fail=0
ok()  { printf '  \033[32mPASS\033[0m  %-32s %s\n' "$1" "${2:-}"; pass=$((pass+1)); }
bad() { printf '  \033[31mFAIL\033[0m  %-32s %s\n' "$1" "${2:-}"; fail=$((fail+1)); }
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1; docker network rm "$NET" >/dev/null 2>&1; rm -rf "$TMP"; }
trap cleanup EXIT

command -v node >/dev/null || { echo "node 22+ required (native WebSocket)"; exit 2; }

echo "── wire-ephemeral · $IMAGE"
echo "   no volume · no compose · no bench · throwaway realm"

# ── 0. THE SHIPPED DEFAULT: no realm must REFUSE, not boot open ──────────────
docker rm -f "$NAME-bare" >/dev/null 2>&1
docker run -d --name "$NAME-bare" -e POSTGRES_PASSWORD=x "$IMAGE" >/dev/null 2>&1
for i in $(seq 1 20); do
  docker logs "$NAME-bare" 2>&1 | grep -q "ociger-ck-identity: REFUSED" && break
  [ "$(docker inspect "$NAME-bare" --format '{{.State.Status}}' 2>/dev/null)" = "exited" ] && break
  sleep 1
done
if docker logs "$NAME-bare" 2>&1 | grep -q "ociger-ck-identity: REFUSED"; then
  ok "bare run REFUSES" "no realm + closed tier → named clause, exit non-zero"
else
  bad "bare run REFUSES" "it booted — the image is not shipping closed"
fi
docker rm -f "$NAME-bare" >/dev/null 2>&1

# ── 1. mint a throwaway Ed25519 realm (pgCK verifies EdDSA only) ─────────────
FIX="$(docker run --rm -e ISS="$ISS" -e AUD="$AUD" -e SUB="$SUB" node:22-slim node -e '
const c=require("crypto"), b=(x)=>Buffer.from(x).toString("base64url");
const {ISS,AUD,SUB}=process.env, k=c.generateKeyPairSync("ed25519"), KID="wire-eph-1";
const jwk=k.publicKey.export({format:"jwk"});
const now=Math.floor(Date.now()/1000);
const h=b(JSON.stringify({alg:"EdDSA",typ:"JWT",kid:KID}));
const p=b(JSON.stringify({iss:ISS,aud:AUD,sub:SUB,iat:now,exp:now+3600}));
process.stdout.write(JSON.stringify({
  jwks: JSON.stringify({keys:[{...jwk,kid:KID,use:"sig",alg:"EdDSA"}]}),
  token: h+"."+p+"."+b(c.sign(null,Buffer.from(h+"."+p),k.privateKey))}));')"
[ -n "$FIX" ] || { echo "could not mint the test realm"; exit 1; }
printf '%s' "$FIX" | python3 -c 'import sys,json;open(sys.argv[1],"w").write(json.load(sys.stdin)["jwks"])' "$TMP/jwks.json"
TOKEN="$(printf '%s' "$FIX" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"],end="")')"
ok "throwaway realm minted" "Ed25519, kid=wire-eph-1"

# ── 2. boot the artifact: realm from ENV, kernel from ENV, NO volume ─────────
docker network create "$NET" >/dev/null 2>&1
docker run -d --name "$NAME" --network "$NET" \
  -p 127.0.0.1:0:9222 -p 127.0.0.1:0:5432 \
  -e POSTGRES_PASSWORD=wireeph \
  -e OCIGER_CK_ADMIT_ANONYMOUS=off \
  -e OCIGER_CK_PROJECT="$KERNEL" \
  -e OCIGER_OIDC_ISSUER="$ISS" \
  -e OCIGER_OIDC_AUDIENCE="$AUD" \
  -e OCIGER_OIDC_JWKS_FILE=/run/jwks.json \
  -v "$TMP/jwks.json:/run/jwks.json:ro" \
  "$IMAGE" >/dev/null 2>&1 || { echo "docker run failed"; exit 1; }

WSPORT="$(docker port "$NAME" 9222/tcp | head -1 | sed 's/.*://')"
PGPORT="$(docker port "$NAME" 5432/tcp | head -1 | sed 's/.*://')"
for i in $(seq 1 60); do
  PGPASSWORD=wireeph psql -h 127.0.0.1 -p "$PGPORT" -U postgres -tAc "SELECT 1" 2>/dev/null | grep -q 1 && break
  sleep 3
done
PGPASSWORD=wireeph psql -h 127.0.0.1 -p "$PGPORT" -U postgres -tAc "SELECT 1" 2>/dev/null | grep -q 1 \
  || { bad "container booted with a realm" "postgres never answered"; echo; echo "wire-ephemeral: $pass passed · $fail failed"; exit 1; }
ok "booted with a realm" "ws:127.0.0.1:$WSPORT (no volume)"

# ── 3. posture + roster delivered by the artifact, from ENV alone ────────────
ROW="$(PGPASSWORD=wireeph psql -h 127.0.0.1 -p "$PGPORT" -U postgres -tAc \
  "SELECT setting||'|'||COALESCE(sourcefile,'-') FROM pg_settings WHERE name='pgck.admit_anonymous';" 2>/dev/null | tr -d '\r')"
case "${ROW##*|}" in
  */run/ck-identity/pgck.conf) ok "posture from delivery chain" "admit_anonymous=${ROW%%|*}" ;;
  *) bad "posture from delivery chain" "owned by ${ROW##*|}" ;;
esac
ROSTER="$(PGPASSWORD=wireeph psql -h 127.0.0.1 -p "$PGPORT" -U postgres -tAc "SHOW pgck.kernels;" 2>/dev/null | tr -d '\r')"
[ "$ROSTER" = "$KERNEL" ] && ok "roster from OCIGER_CK_PROJECT" "$ROSTER" \
                          || bad "roster from OCIGER_CK_PROJECT" "got '$ROSTER', wanted '$KERNEL'"

# ── 4. admission: anonymous REFUSED, verified ADMITTED ───────────────────────
A="$(node "$HERE/lib/wire-probe.mjs" "ws://127.0.0.1:$WSPORT" - "$KERNEL" surface.check 2>/dev/null)"
case "$A" in *closed_1008*) ok "anonymous refused" "close 1008" ;; *) bad "anonymous refused" "$A" ;; esac

# ── 5. THE WIRE SET — a verified dispatch must answer, verb by verb ──────────
for v in surface.check affordances fleet.adoptions authority.mine adoption.check; do
  R="$(node "$HERE/lib/wire-probe.mjs" "ws://127.0.0.1:$WSPORT" "$TOKEN" "$KERNEL" "$v" 2>/dev/null)"
  case "$R" in
    '{'*) ok "wire $v" "$(printf '%s' "$R" | cut -c1-48)…" ;;
    *)    bad "wire $v" "$R" ;;
  esac
done

# ── 6. the identity was SERVER-DERIVED — broker log, not the client ──────────
docker logs "$NAME" 2>&1 | grep -q "ADMIT verified sub=$SUB" \
  && ok "broker admitted the identity" "sub=$SUB" \
  || bad "broker admitted the identity" "no ADMIT line — the client view is not the oracle"

echo
echo "──────────────────────────────────────────────────────────────────────"
echo "wire-ephemeral: $pass passed · $fail failed   ($IMAGE)"
[ "$fail" -eq 0 ] || exit 1
