#!/usr/bin/env bash
# GREEN (v0.7.43+): an incoherent identity posture is REFUSED before postgres
# starts, rather than served as a door that is open or dead.
#
# ociger-ck-identity is an s6 ONESHOT that runs before postgres and nats — the
# cheap place to refuse. A bad posture caught there costs a failed boot with a
# named clause; the same posture served costs a door diagnosable only from the
# broker log, which no consumer can read.
#
# THE DENY-ALL QUADRANT is why this case exists and why it ships in the same cut
# as the posture change. Emitting admit_anonymous=off under a realm is right,
# but it turns a previously-harmless misconfiguration into a fatal one: before,
# an unusable JWKS degraded to the anonymous tier; after, it means "verify
# everything, with no key to verify against" — every connection refused,
# including valid tokens. The refusals are what stop the fix shipping that trap.
#
# Runs its own throwaway containers (og- prefixed, removed on exit): the case is
# about BOOT, so it cannot use the suite's already-running one.
source "$(dirname "$0")/../lib.sh"

IMG="${OG_TDD_IMAGE:-ghcr.io/sporaxis-com/ociger-ck-allinone:latest}"
NAME="og-tdd-posture-$$"
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

# boot_with <env...> -> prints the container's identity log lines and sets RC to
# whether ociger-ck-identity refused. A refusal is a non-zero oneshot exit, which
# S6_BEHAVIOUR_IF_STAGE2_FAILS=2 turns into a stopped container.
refused_with() {
  cleanup
  docker run -d --name "$NAME" -e POSTGRES_PASSWORD=ogtdd "$@" "$IMG" >/dev/null 2>&1 || return 2
  # THE ORACLE IS WHETHER IT SERVES, NOT WHAT IT PRINTS. An earlier revision
  # grepped only for the string "REFUSED" and reported BROKEN on a container
  # that had refused correctly — the empty-declaration path returned its error
  # through log.Fatalf and never printed that word. Testing the log text tested
  # the log text. Container state answers the actual question; the text is
  # corroboration, and is asserted separately so a silent exit still fails.
  local i state
  for i in $(seq 1 25); do
    docker logs "$NAME" 2>&1 | grep -q "ociger-ck-identity: POSTURE" && return 1
    state="$(docker inspect "$NAME" --format '{{.State.Status}}' 2>/dev/null)"
    if [ "$state" = "exited" ]; then
      # refused: the oneshot failed, so s6 stopped the container before postgres
      docker logs "$NAME" 2>&1 | grep -q "ociger-ck-identity:.*REFUSED" && return 0
      echo "  note: $NAME exited without a REFUSED line — a silent refusal is still a defect" >&2
      return 4
    fi
    sleep 1
  done
  return 3
}

# (a) THE SHIPPED DEFAULT with no realm. Since v0.7.43 the image bakes
#     OCIGER_CK_ADMIT_ANONYMOUS=off, so this is what a bare `docker run` hits:
#     a door closed to unverified connections with nothing to verify against.
#     It must refuse and name the JWKS path, not boot into a dead door.
refused_with
case $? in
  0) ;;
  1) RED "a bare container BOOTED — the image is not shipping closed (OCIGER_CK_ADMIT_ANONYMOUS=off), or the refusal is missing" ;;
  4) BROKEN "(a) bare run refused SILENTLY — it exited without naming a clause; the message is the documentation here" ;;
  *) BROKEN "could not determine the outcome for (a) bare-run-with-no-realm" ;;
esac

# (b) realm materials supplied that do not ground a verifier: with the tier
#     closed this is deny-all (even a valid token refused), and with it open
#     every token is silently downgraded. Neither is a door.
refused_with -e OCIGER_OIDC_ISSUER=https://example.invalid/realms/x
case $? in
  0) ;;
  1) RED "a partially configured realm (issuer only, no audience/JWKS) was SERVED, not refused" ;;
  4) BROKEN "(b) partial realm refused SILENTLY — no clause named" ;;
  *) BROKEN "could not determine the outcome for (b) partial-realm" ;;
esac

# (c) an emptied declaration must not silently fall back to a default.
refused_with -e OCIGER_CK_ADMIT_ANONYMOUS=
case $? in
  0) ;;
  1) RED "OCIGER_CK_ADMIT_ANONYMOUS= (emptied) was SERVED — something is supplying a fallback, which is the defect this cut removed" ;;
  4) BROKEN "(c) emptied declaration refused SILENTLY — no clause named" ;;
  *) BROKEN "could not determine the outcome for (c) emptied-declaration" ;;
esac

# THE MIRROR: a coherent posture must still boot. A refusal gate that refuses
# everything would pass all three checks above and be worthless.
refused_with -e OCIGER_CK_ADMIT_ANONYMOUS=on
case $? in
  1) GREEN "incoherent postures refused at boot with a named clause; an explicitly declared anonymous tier still boots" ;;
  0) BROKEN "a container declaring OCIGER_CK_ADMIT_ANONYMOUS=on was REFUSED — the gate is over-refusing, which is worse than the defect it guards" ;;
  *) BROKEN "could not determine the outcome for the coherent-boot mirror" ;;
esac
