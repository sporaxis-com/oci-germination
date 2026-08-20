#!/bin/bash
# verify-callout.sh — verified-path proof for ck-allinone v0.7.30 (og#19).
#
# Boots the bundle with a SELF-CONTAINED test realm (a fresh Ed25519 key plays the
# OIDC issuer; its JWK is delivered via OCIGER_OIDC_JWKS) and proves the pgCK
# auth-callout end-to-end over WSS — no external IdP. Mirrors pgCK's
# scripts/dev-callout-e2e.sh (SPEC.SECURITY §7), asserting:
#
#   A  valid token → dispatch on input.kernel.pgck.id.<sub>.action.task.create
#                    → result ok:true AND the sealed event carries the verified
#                      identity (urn:ckp:participant:<sub>)  [hops 4→6]
#   B  no token    → anonymous: cannot publish input.*  (subscribe-only)
#   C  forged token→ same as anonymous (fail-open-to-anonymous, never-to-admitted)
#   D  valid token, SOMEONE ELSE'S id segment → broker denies; nothing seals
#
# Usage: scripts/verify-callout.sh [IMAGE]
set -euo pipefail

IMAGE="${1:-ociger-ck-allinone:v0.7.30-local}"
NET="ckcallout-net"
C="ckcallout-verify"
ISS="https://realm.test/"
AUD="ck-allinone"
SUB="alice-e2e"

cleanup() { docker rm -f "$C" >/dev/null 2>&1 || true; docker network rm "$NET" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

echo "════════════════════════════════════════════════════════════"
echo "[verify-callout] verified-path proof — image: $IMAGE"
echo "════════════════════════════════════════════════════════════"

# ── 1. Mint the test realm: Ed25519 key → JWKS + valid token + forged token ──
# EdDSA JWT per pgCK src/jwt_verify.rs: header {alg:EdDSA,kid}, claims
# {iss,aud,sub,exp}; JWKS {kty:OKP,crv:Ed25519,kid,x}. Emitted as one JSON blob.
FIX="$(docker run --rm -e ISS="$ISS" -e AUD="$AUD" -e SUB="$SUB" node:22-slim node -e '
const crypto = require("crypto");
const b64u = (b) => Buffer.from(b).toString("base64url");
const {ISS, AUD, SUB} = process.env;
const realm = crypto.generateKeyPairSync("ed25519");
const foreign = crypto.generateKeyPairSync("ed25519");
const KID = "test-realm-1";
const jwk = realm.publicKey.export({format:"jwk"});            // {kty:OKP,crv:Ed25519,x}
const jwks = JSON.stringify({keys:[{...jwk, kid:KID, use:"sig", alg:"EdDSA"}]});
const now = Math.floor(Date.now()/1000);
function mint(key, kid){
  const h = b64u(JSON.stringify({alg:"EdDSA", typ:"JWT", kid}));
  const p = b64u(JSON.stringify({iss:ISS, aud:AUD, sub:SUB, iat:now, exp:now+3600}));
  const sig = crypto.sign(null, Buffer.from(h+"."+p), key);    // ed25519 → algorithm null
  return h+"."+p+"."+b64u(sig);
}
process.stdout.write(JSON.stringify({
  jwks,
  valid:  mint(realm.privateKey, KID),
  forged: mint(foreign.privateKey, KID),   // right kid, WRONG signing key → sig fails
}));
')"
JWKS="$(printf '%s' "$FIX" | python3 -c 'import sys,json;print(json.load(sys.stdin)["jwks"],end="")')"
TOKV="$(printf '%s' "$FIX" | python3 -c 'import sys,json;print(json.load(sys.stdin)["valid"],end="")')"
TOKF="$(printf '%s' "$FIX" | python3 -c 'import sys,json;print(json.load(sys.stdin)["forged"],end="")')"
echo "[verify-callout] test realm minted (issuer=$ISS aud=$AUD sub=$SUB)"

# ── 2. Boot the bundle with the realm (callout ACTIVE) ──
docker network create "$NET" >/dev/null
docker run -d --name "$C" --network "$NET" \
  -e OCIGER_OIDC_JWKS="$JWKS" \
  -e OCIGER_OIDC_ISSUER="$ISS" \
  -e OCIGER_OIDC_AUDIENCE="$AUD" \
  "$IMAGE" >/dev/null
echo "[verify-callout] booting…"

# Readiness: the in-extension bgworker prints "responder live on $SYS.REQ.USER.AUTH"
# once the callout is serving. Our launcher's `postgres --single` init phase does
# NOT run bgworkers, so this line comes only from the real server (no temp-server
# false positive, unlike a restart-based compose).
READY=""
for i in $(seq 1 45); do
  if docker logs "$C" 2>&1 | grep -q "responder live on"; then READY=1; break; fi
  sleep 2
done
if [ -z "$READY" ]; then
  echo "✗ responder never came up"; docker logs "$C" 2>&1 | grep -iE 'pgck|nats|responder|ck-identity|oidc' | tail -25; exit 1
fi
sleep 3   # let the demo board settle (init.sql ckp.boot ran at initdb)
echo "[verify-callout] responder live"

# ── 3. Assertions over WSS (ws npm package → Buffer frames; native global
#       WebSocket delivers Blobs that don't .toString(), so we use ws) ──
ASSERT_JS="$(cat <<'JSEOF'
const { WebSocket } = require("ws");
const { TOKV, TOKF, SUB, NATS_HOST } = process.env;
function run(token, subj, payload, collectMs) {
  return new Promise((resolve) => {
    const ws = new WebSocket("ws://" + NATS_HOST + ":9222");
    let buf = "", denied = false;
    const t = setTimeout(() => { try { ws.close(); } catch (e) {} resolve({ buf, denied }); }, collectMs);
    ws.on("message", (d) => {
      const x = d.toString(); buf += x;
      if (x.startsWith("INFO ")) {
        const c = token
          ? 'CONNECT {"verbose":false,"pedantic":false,"protocol":1,"headers":true,"auth_token":"' + token + '"}\r\n'
          : 'CONNECT {"verbose":false,"pedantic":false,"protocol":1,"headers":true}\r\n';
        ws.send(c);
        ws.send("SUB result.kernel.pgck.> 1\r\n");
        ws.send("SUB event.kernel.pgck.> 2\r\n");
        if (payload) setTimeout(() => ws.send("PUB " + subj + " " + payload.length + "\r\n" + payload + "\r\n"), 400);
      }
      if (x.includes("Permissions Violation") || x.includes("Authorization Violation")) denied = true;
    });
    ws.on("error", () => { clearTimeout(t); resolve({ buf, denied }); });
  });
}
(async () => {
  const out = {};
  const bad    = await run(TOKV, "input.kernel.pgck.id.someone-else.action.task.create", JSON.stringify({task:{target_kernel:"demo",title:"must-not-seal"}}), 4500);
  const ok     = await run(TOKV, "input.kernel.pgck.id." + SUB + ".action.task.create", JSON.stringify({task:{target_kernel:"demo",title:"verified-ok"}}), 6500);
  const anon   = await run("",   "input.kernel.pgck.id." + SUB + ".action.task.create", JSON.stringify({task:{target_kernel:"demo",title:"anon"}}), 4000);
  const forged = await run(TOKF, "input.kernel.pgck.id." + SUB + ".action.task.create", JSON.stringify({task:{target_kernel:"demo",title:"forged"}}), 4000);
  out.A_ok_true        = /"ok"\s*:\s*true/.test(ok.buf);
  out.A_sealed_by      = ok.buf.includes("urn:ckp:participant:" + SUB);
  out.D_foreign_sealed = bad.buf.includes("must-not-seal") && /Task\.sealed/.test(bad.buf);
  out.B_anon_denied    = anon.denied || !/"ok"\s*:\s*true/.test(anon.buf);
  out.C_forged_denied  = forged.denied || !/"ok"\s*:\s*true/.test(forged.buf);
  process.stdout.write(JSON.stringify(out));
})();
JSEOF
)"

RESULT="$(docker run --rm --network "$NET" \
  -e TOKV="$TOKV" -e TOKF="$TOKF" -e SUB="$SUB" -e NATS_HOST="$C" -e ASSERT_JS="$ASSERT_JS" \
  node:22-slim sh -c 'cd /tmp && npm install --silent --no-save ws@8 >/dev/null 2>&1 && printf "%s" "$ASSERT_JS" > /tmp/assert.js && node /tmp/assert.js' 2>/dev/null || echo '{}')"

echo "[verify-callout] raw: $RESULT"
pass() { echo "$RESULT" | python3 -c "import sys,json;d=json.load(sys.stdin);w=('$2'=='true');sys.exit(0 if d.get('$1')==w else 1)" 2>/dev/null; }

FAIL=0
if pass A_ok_true true;         then echo "   ✓ A1 verified dispatch → ok:true"; else echo "   ✗ A1 verified dispatch did NOT return ok:true"; FAIL=1; fi
if pass A_sealed_by true;       then echo "   ✓ A2 sealed event carries urn:ckp:participant:$SUB (verified identity)"; else echo "   ✗ A2 sealed event lacks the verified identity"; FAIL=1; fi
if pass B_anon_denied true;     then echo "   ✓ B  anonymous cannot dispatch (subscribe-only)"; else echo "   ✗ B  anonymous was able to dispatch"; FAIL=1; fi
if pass C_forged_denied true;   then echo "   ✓ C  forged token drops to anonymous (no dispatch)"; else echo "   ✗ C  forged token was admitted"; FAIL=1; fi
if pass D_foreign_sealed false; then echo "   ✓ D  foreign id segment denied (nothing sealed)"; else echo "   ✗ D  a foreign-id publish sealed"; FAIL=1; fi

if [ "$FAIL" != 0 ]; then
  echo "✗ verify-callout FAILED"; docker logs "$C" 2>&1 | grep -iE 'pgck|nats|responder|oidc|requester|admit' | tail -30; exit 1
fi
echo "[verify-callout] ✓ ALL PASS — verified identity flows end-to-end (hops 4→6)"
