// Command ociger-ck-identity is the boot-time provisioner for ck-allinone's NATS
// admittance (bundle v0.7.30). It runs as an s6 oneshot BEFORE the nats and
// postgres longruns and writes /etc/nats/nats-server.conf plus the
// /run/ck-identity/pgck.conf fragment postgres pulls in via include_if_exists.
//
// It has two modes, chosen by whether an OIDC realm is configured:
//
//	REALM (OCIGER_OIDC_JWKS + OCIGER_OIDC_ISSUER + OCIGER_OIDC_AUDIENCE all set):
//	  activate the auth-callout. Resolve the callout ACCOUNT nkey
//	  (OCIGER_NATS_ACCOUNT_SEED if the operator supplied one → stable, else
//	  minted per boot) and write nats-server.conf with the auth_callout stanza
//	  (issuer = the account public key; pgck_worker the sole bypass user) plus
//	  pgck.conf with pgck.nats_account_seed, the worker-cred nats_url, and the
//	  pgck.oidc_* verifier config. pgCK then verifies bearers against the realm
//	  and binds the verified sub to ckp.requester (hop 4). Anonymous connections
//	  are admitted subscribe-only.
//
//	NO REALM (default): no auth-callout, no account seed — pgCK's responder is
//	  not started (admission unchanged) and anonymous connections may dispatch
//	  (the v0.7.29 behavior). This is the zero-config default for adopters
//	  without an identity plane; a client cannot forge a governed *.sealed event
//	  (the interim publish-deny is retained).
//
// The account seed is NEVER baked into the image: absent an operator seed it is
// minted fresh each boot and lives only in tmpfs (/run) and process memory.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nats-io/nkeys"
)

const (
	defaultNatsConf = "/etc/nats/nats-server.conf"
	defaultPgckConf = "/run/ck-identity/pgck.conf"
	workerUser      = "pgck_worker"
	natsCorePort    = 4222
	natsWssPort     = 9222
	// pgck.conf carries the account SEED and is read ONLY by postgres (uid 999),
	// which pulls it via include_if_exists; the in-process pgCK bgworker then
	// consumes the GUCs. v0.7.31 hardening chowns it to postgres so the
	// dropped-to-non-root nats/httpd users cannot read the seed.
	postgresUID = 999
	postgresGID = 999
)

// oidcConfig is the operator-supplied realm the callout verifies bearers against.
//
// `jwks` is the JWKS **DOCUMENT** (JSON), never a URL. pgCK verifies bearers
// in-memory and performs no egress in the live path, so a URL in pgck.oidc_jwks
// can never verify anything — the responder starts, token verify stays off, and
// every connection lands in the anonymous tier (og#29). The document is INJECTED
// at container start; this provisioner never fetches it, because it runs before
// nats and postgres and a network call here would put DNS and realm availability
// into the boot path of a bundle that is otherwise egress-free.
type oidcConfig struct {
	jwks       string // the JWKS document, or "" when it could not be resolved
	jwksSource string // where it came from, for the boot log
	issuer     string
	audience   string
}

func main() {
	natsConf := getenv("OCIGER_NATS_CONF", defaultNatsConf)
	pgckConf := getenv("OCIGER_PGCK_CONF", defaultPgckConf)

	oidc, realm := resolveOIDC()
	oidcSet := oidcVarsSet()

	admitAnon, err := resolveAdmitAnonymous()
	if err != nil {
		log.Fatalf("ociger-ck-identity: %v", err)
	}

	// ── BOOT REFUSALS (v0.7.43) ─────────────────────────────────────────────
	// This process is an s6 ONESHOT before postgres and nats — the cheap place
	// to refuse. A bad posture caught here costs a failed boot with a named
	// clause; the same posture SERVED costs a door that is silently open or
	// silently dead, diagnosable only from a broker log no consumer can read.
	//
	// The posture is expressed by exactly two facts, and neither is derived:
	// the tier is OCIGER_CK_ADMIT_ANONYMOUS (shipped `off`), and whether a
	// verifier exists is whether the realm resolves. There is no third
	// declaration to disagree with them.
	switch {
	// The door is closed and there is nothing to open it with. Since the image
	// ships admit_anonymous=off, THIS is what a bare `docker run` now hits, and
	// the message is the documentation: it names the intended path first.
	case !realm && admitAnon == "off":
		log.Fatalf("ociger-ck-identity: REFUSED — this door is closed to unverified connections " +
			"(OCIGER_CK_ADMIT_ANONYMOUS=off, the shipped default) but NO REALM IS CONFIGURED, so " +
			"nothing can ever be verified and every connection would be refused.\n" +
			"  DELIVER A JWKS — this is the intended path:\n" +
			"    -e OCIGER_OIDC_ISSUER=<your realm issuer>\n" +
			"    -e OCIGER_OIDC_AUDIENCE=<the audience your tokens carry>\n" +
			"    -e OCIGER_OIDC_JWKS_FILE=/run/jwks.json  -v /path/to/jwks.json:/run/jwks.json:ro\n" +
			"  The JWKS must be the DOCUMENT, never a URL (this bundle never fetches), and must\n" +
			"  carry at least one Ed25519 key — pgCK verifies EdDSA only.\n" +
			"  For local development WITHOUT identity, opt in explicitly: " +
			"-e OCIGER_CK_ADMIT_ANONYMOUS=on (every connection is then UNVERIFIED and seals carry " +
			"no identity).")

	// Realm materials were supplied and did not ground a verifier. Serving this
	// with the tier closed is the deny-all quadrant — even a valid token is
	// refused — and serving it with the tier open silently downgrades every
	// token. Neither is a door, so refuse and name what is missing.
	case len(oidcSet) > 0 && !realm:
		missing := []string{}
		if strings.TrimSpace(oidc.issuer) == "" {
			missing = append(missing, "OCIGER_OIDC_ISSUER")
		}
		if strings.TrimSpace(oidc.audience) == "" {
			missing = append(missing, "OCIGER_OIDC_AUDIENCE")
		}
		if oidc.jwks == "" {
			missing = append(missing, "a USABLE JWKS document (see the JWKS UNUSABLE line above)")
		}
		log.Fatalf("ociger-ck-identity: REFUSED — %s set, but the realm does not ground a verifier; "+
			"missing: %s. A partially configured realm is not a door: with the anonymous tier closed "+
			"every connection is refused, and with it open every token is silently downgraded.",
			strings.Join(oidcSet, ", "), strings.Join(missing, ", "))
	}

	if !realm {
		if err := writeFileMode(natsConf, anonNatsServerConf(), 0o644); err != nil {
			log.Fatalf("ociger-ck-identity: write %s: %v", natsConf, err)
		}
		if err := writePgckConf(pgckConf, anonPgckConf(admitAnon)); err != nil {
			log.Fatalf("ociger-ck-identity: write %s: %v", pgckConf, err)
		}
		log.Printf("ociger-ck-identity: POSTURE anonymous — callout off, token verify OFF, "+
			"pgck.admit_anonymous=%s (OCIGER_CK_ADMIT_ANONYMOUS). Every connection is admitted UNVERIFIED and seals "+
			"carry no identity. To run a door, set OCIGER_OIDC_ISSUER + _AUDIENCE + _JWKS_FILE. "+
			"Wrote %s + %s", admitAnon, natsConf, pgckConf)
		return
	}

	seed, pub, err := resolveAccount(os.Getenv("OCIGER_NATS_ACCOUNT_SEED"))
	if err != nil {
		log.Fatalf("ociger-ck-identity: account key: %v", err)
	}
	workerPass, err := resolveWorkerPassword(os.Getenv("OCIGER_NATS_WORKER_PASSWORD"))
	if err != nil {
		log.Fatalf("ociger-ck-identity: worker password: %v", err)
	}

	// nats-server.conf stays 0644 — the dropped-to-non-root `nats` user must read
	// it (issuer pubkey + worker password; NOT the seed). pgck.conf carries the
	// account SEED, so writePgckConf writes it 0640 and chowns it to postgres
	// (999) — the only reader (postgres → include_if_exists → GUC → in-process
	// pgCK bgworker). The dropped `nats`/`httpd` users cannot read the seed.
	if err := writeFileMode(natsConf, natsServerConf(pub, workerPass), 0o644); err != nil {
		log.Fatalf("ociger-ck-identity: write %s: %v", natsConf, err)
	}
	if err := writePgckConf(pgckConf, pgckSecretsConf(seed, workerPass, oidc, admitAnon)); err != nil {
		log.Fatalf("ociger-ck-identity: write %s: %v", pgckConf, err)
	}

	source := "minted per-boot (ephemeral)"
	if strings.TrimSpace(os.Getenv("OCIGER_NATS_ACCOUNT_SEED")) != "" {
		source = "operator-supplied"
	}
	// ONE posture block. The audience and the usable-key census are printed
	// because they are the two facts every identity diagnosis on this fleet has
	// needed and neither was ever in the log: an audience mismatch previously
	// cost several wire probes plus a JWKS decode, and a token refused for the
	// wrong key looks identical to one refused for the wrong audience.
	usable, kids := keyCensus(oidc.jwks)
	kidList := "none"
	if len(kids) > 0 {
		kidList = strings.Join(kids, ", ")
	}
	tier := "unverified connections are REFUSED, not downgraded"
	if admitAnon == "on" {
		tier = "MIXED TIER: unverified connections are DOWNGRADED to anonymous (subscribe-only, no publish), not refused"
	}
	log.Printf("ociger-ck-identity: POSTURE realm — auth-callout ACTIVE, "+
		"pgck.admit_anonymous=%s (OCIGER_CK_ADMIT_ANONYMOUS) — %s\n"+
		"  issuer   : %s\n"+
		"  audience : %s   ← a token whose aud does not carry this is REFUSED\n"+
		"  jwks     : %d usable Ed25519 key(s) [kid: %s] from %s (%d bytes, never fetched)\n"+
		"  account  : %s [%s]\n"+
		"  wrote    : %s + %s",
		admitAnon, tier,
		oidc.issuer, oidc.audience, usable, kidList, oidc.jwksSource, len(oidc.jwks),
		pub, source, natsConf, pgckConf)
}

// resolveOIDC reads the realm config. All three fields are required — a partial
// config is a misconfiguration, treated as "no realm" (anonymous mode).
//
// The JWKS is resolved to a DOCUMENT (§resolveJWKS). A configured-but-unusable
// JWKS does NOT drop the bundle out of realm mode: the callout still activates
// and pgCK reports "token verify: off -> anonymous". That is the deliberate
// degrade — service alive, identity off, one legible log line. Dropping to
// anonymous mode instead would also disable the responder, and pairing an
// unusable value with strict admittance would refuse every connection.
func resolveOIDC() (oidcConfig, bool) {
	c := oidcConfig{
		issuer:   strings.TrimSpace(os.Getenv("OCIGER_OIDC_ISSUER")),
		audience: strings.TrimSpace(os.Getenv("OCIGER_OIDC_AUDIENCE")),
	}
	configured := strings.TrimSpace(os.Getenv("OCIGER_OIDC_JWKS")) != "" ||
		strings.TrimSpace(os.Getenv("OCIGER_OIDC_JWKS_FILE")) != ""

	doc, src, err := resolveJWKS()
	if err != nil {
		// Never write a value that cannot verify. Omit the GUC and say why.
		log.Printf("ociger-ck-identity: JWKS UNUSABLE — %v", err)
		log.Printf("ociger-ck-identity: pgck.oidc_jwks will be OMITTED; the callout activates and " +
			"pgCK reports token verify OFF (anonymous tier). Deliver the JWKS DOCUMENT at container " +
			"start via OCIGER_OIDC_JWKS_FILE (preferred) or OCIGER_OIDC_JWKS. This bundle never fetches it.")
	}
	c.jwks, c.jwksSource = doc, src

	return c, configured && c.issuer != "" && c.audience != ""
}

// resolveJWKS returns the JWKS DOCUMENT, injected at container start.
//
// Order: OCIGER_OIDC_JWKS_FILE (a path placed into the container) wins over
// OCIGER_OIDC_JWKS (the document inline). A URL in either is refused by name —
// it is the single most likely mistake and it fails silently downstream.
//
// This function performs NO network I/O and this package imports no HTTP client.
// That is a contract, not an implementation detail: see the type comment.
func resolveJWKS() (doc string, source string, err error) {
	if p := strings.TrimSpace(os.Getenv("OCIGER_OIDC_JWKS_FILE")); p != "" {
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return "", "", fmt.Errorf("OCIGER_OIDC_JWKS_FILE=%s: %w", p, readErr)
		}
		v, vErr := validateJWKS(string(b))
		if vErr != nil {
			return "", "", fmt.Errorf("OCIGER_OIDC_JWKS_FILE=%s: %w", p, vErr)
		}
		return v, "file " + p, nil
	}
	if s := strings.TrimSpace(os.Getenv("OCIGER_OIDC_JWKS")); s != "" {
		v, vErr := validateJWKS(s)
		if vErr != nil {
			return "", "", fmt.Errorf("OCIGER_OIDC_JWKS: %w", vErr)
		}
		return v, "env OCIGER_OIDC_JWKS", nil
	}
	return "", "", fmt.Errorf("no JWKS supplied (set OCIGER_OIDC_JWKS_FILE or OCIGER_OIDC_JWKS)")
}

// validateJWKS accepts only a JWKS document carrying at least one key, and
// returns it collapsed to a single line (postgresql.conf values are one line).
func validateJWKS(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty")
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return "", fmt.Errorf("carries a URL, not the JWKS document — pgCK never fetches " +
			"(no egress in the live path); deliver the JSON itself")
	}
	var parsed struct {
		Keys []struct {
			Kty string `json:"kty"`
			Crv string `json:"crv"`
		} `json:"keys"`
	}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return "", fmt.Errorf("not valid JSON: %w", err)
	}
	if len(parsed.Keys) == 0 {
		return "", fmt.Errorf("parsed as JSON but carries no \"keys\" — not a JWKS")
	}
	// A JWKS pgCK cannot USE is worse than none: it would be written to
	// pgck.oidc_jwks, this process would log "token verify: ON", and pgCK would
	// then load zero keys and admit every connection to the anonymous tier —
	// two layers both reporting healthy while nobody is ever verified.
	//
	// pgCK's verifier is EdDSA-only: it refuses any header whose `alg` is not
	// EdDSA, and its JWKS parser keeps only kty:OKP + crv:Ed25519, skipping
	// RSA/EC entries and erroring when none remain. Realms legitimately publish
	// mixed key sets — the bench realm ships EdDSA + RS256 + RSA-OAEP — so this
	// requires at least ONE usable key rather than rejecting the others.
	//
	// An RSA-only set (Azure Entra signs RS256 and offers no EdDSA) is refused
	// here, by name, at boot — instead of silently degrading at admission.
	usable := 0
	for _, k := range parsed.Keys {
		if k.Kty == "OKP" && k.Crv == "Ed25519" {
			usable++
		}
	}
	if usable == 0 {
		return "", fmt.Errorf("carries %d key(s) but none are Ed25519 (kty:OKP, crv:Ed25519) — "+
			"pgCK's auth-callout verifies EdDSA only and would load no key from this document, "+
			"leaving every connection anonymous. An RSA-only realm (e.g. Azure Entra, RS256) "+
			"cannot ground this bundle", len(parsed.Keys))
	}
	compact := &bytes.Buffer{}
	if err := json.Compact(compact, []byte(s)); err != nil {
		return "", fmt.Errorf("could not compact: %w", err)
	}
	return compact.String(), nil
}

// resolveAccount returns the account seed (canonical string form, e.g. "SA...")
// and its public key. A non-empty seedEnv MUST parse as an ACCOUNT nkey seed
// (operator override → stable identity); otherwise a fresh account is minted.
func resolveAccount(seedEnv string) (seed string, pub string, err error) {
	var kp nkeys.KeyPair
	if s := strings.TrimSpace(seedEnv); s != "" {
		kp, err = nkeys.FromSeed([]byte(s))
		if err != nil {
			return "", "", fmt.Errorf("OCIGER_NATS_ACCOUNT_SEED is not a valid nkey seed: %w", err)
		}
	} else {
		kp, err = nkeys.CreateAccount()
		if err != nil {
			return "", "", err
		}
	}
	pub, err = kp.PublicKey()
	if err != nil {
		return "", "", err
	}
	// Account public keys are prefixed 'A'. Reject a user/operator/server seed —
	// the callout issuer must be an account, or the broker rejects every mint.
	if !strings.HasPrefix(pub, "A") {
		return "", "", fmt.Errorf("resolved key is not an ACCOUNT nkey (public key %q must start with 'A')", pub)
	}
	sb, err := kp.Seed()
	if err != nil {
		return "", "", err
	}
	return string(sb), pub, nil
}

// resolveWorkerPassword returns the operator-supplied password verbatim, or a
// fresh 24-byte random hex string.
func resolveWorkerPassword(envPass string) (string, error) {
	if p := strings.TrimSpace(envPass); p != "" {
		return p, nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// natsServerConf renders the server-config-mode auth_callout config (REALM mode).
// Every connection that is NOT an auth_user is routed to the in-extension
// responder, which admits a verified bearer as its id-scoped identity or (absent/
// invalid) as anonymous subscribe-only.
func natsServerConf(issuerPub, workerPass string) string {
	return fmt.Sprintf(`# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.
# REALM mode: auth-callout active. issuer = the resolved account public key.
port: %d
websocket {
  port: %d
  no_tls: true
}
authorization {
  users: [
    { user: %s, password: %q }
  ]
  auth_callout {
    issuer: %q
    auth_users: [ %s ]
  }
}
`, natsCorePort, natsWssPort, workerUser, workerPass, issuerPub, workerUser)
}

// anonNatsServerConf renders the no-realm config: no callout, every connection
// maps to the anon no_auth_user, denied only publish on input.*.*.sealed so a
// client cannot forge a governed sealed event.
func anonNatsServerConf() string {
	return fmt.Sprintf(`# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.
# ANONYMOUS mode: no OIDC realm configured, so no auth-callout.
port: %d
websocket {
  port: %d
  no_tls: true
}
authorization {
  users: [
    { user: anon, password: anon,
      permissions: { publish: { deny: "input.kernel.demo.action.*.sealed" } } }
  ]
}
no_auth_user: anon
`, natsCorePort, natsWssPort)
}

// pgckSecretsConf renders the postgresql.conf fragment for REALM mode: the
// account seed, worker-cred nats_url, and the pgck.oidc_* verifier config. Values
// are single-quoted; embedded single quotes are doubled (postgresql.conf escaping).
// ── POSTURE ──────────────────────────────────────────────────────────────
//
// pgCK has six identity GUCs. Five are SAFE-BY-EMPTINESS: absent means inert.
// pgck.admit_anonymous is UNSAFE-BY-ABSENCE — its built-in default is `true`,
// so a provisioner written the only sane way (emit every key that needs a
// value) emits the five and leaves the door OPEN under a fully configured
// realm. The sixth key is structurally invisible to that method, which is why
// this was a design gap rather than an oversight.
//
// It is emitted EXPLICITLY in both branches from v0.7.43. pgck.conf lives on
// tmpfs and is regenerated at every container start, so the value is owned by
// the delivery chain and cannot be reverted by a volume wipe — unlike an
// operator's ALTER SYSTEM, which lands in PGDATA/postgresql.auto.conf and dies
// with the next `down -v`.
// resolveAdmitAnonymous reads the posture from the environment. It DERIVES
// NOTHING.
//
// The image bakes ENV OCIGER_CK_ADMIT_ANONYMOUS=off, so the default is visible
// in `docker inspect` rather than living here. An earlier revision computed the
// value from whether a realm resolved — which put the default back in code, the
// exact shape this work exists to remove (`source: default` is an unowned
// value; a default in Go is an unreadable one).
//
// ⚠ THE ANONYMOUS TIER IS A CAPABILITY, NOT A DEFECT. pgCK documents it:
//
//	Admission::Anonymous => pub_allow: []                    // deny all publish
//	                        sub_allow: event.kernel.<k>.>    // read-only subscriber
//
// A deployment running a realm PLUS a public read-only tier sets `on` and keeps
// it. What v0.7.43 changed is only which way the SHIPPED default points: closed,
// so an operator who configures nothing gets a door that refuses rather than one
// that admits everyone.
//
// Empty is refused rather than defaulted: if the baked ENV has been cleared, the
// operator removed the declaration and must restate it.
func resolveAdmitAnonymous() (string, error) {
	switch v := strings.ToLower(strings.TrimSpace(os.Getenv("OCIGER_CK_ADMIT_ANONYMOUS"))); v {
	case "on", "true", "1", "yes":
		return "on", nil
	case "off", "false", "0", "no":
		return "off", nil
	case "":
		return "", fmt.Errorf("REFUSED — OCIGER_CK_ADMIT_ANONYMOUS is empty. The image ships it as `off`; " +
			"clearing it removes the posture declaration entirely, and this bundle will not guess " +
			"one. Set it to off (a door: unverified connections refused) or on (the mixed tier: " +
			"unverified downgraded to subscribe-only)")
	default:
		return "", fmt.Errorf("REFUSED — OCIGER_CK_ADMIT_ANONYMOUS=%q is not a boolean: expected on|off", v)
	}
}

// oidcVarsSet reports which OCIGER_OIDC_* variables carry a value. Used by the
// coherence refusals: an operator who declared `anonymous` while setting realm
// variables believes they configured a realm and has not.
func oidcVarsSet() []string {
	var set []string
	for _, k := range []string{"OCIGER_OIDC_ISSUER", "OCIGER_OIDC_AUDIENCE", "OCIGER_OIDC_JWKS_FILE", "OCIGER_OIDC_JWKS"} {
		if strings.TrimSpace(os.Getenv(k)) != "" {
			set = append(set, k)
		}
	}
	return set
}

// keyCensus counts the keys pgCK can actually VERIFY with and returns their
// kids. Printed at boot so an audience or key mismatch costs a grep rather than
// a wire probe: the two most expensive identity diagnoses on this fleet were
// both "which key / which audience", and neither was in the log.
func keyCensus(doc string) (usable int, kids []string) {
	var parsed struct {
		Keys []struct {
			Kty string `json:"kty"`
			Crv string `json:"crv"`
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	if json.Unmarshal([]byte(doc), &parsed) != nil {
		return 0, nil
	}
	for _, k := range parsed.Keys {
		if k.Kty == "OKP" && k.Crv == "Ed25519" {
			usable++
			if k.Kid != "" {
				kids = append(kids, k.Kid)
			}
		}
	}
	return usable, kids
}

func pgckSecretsConf(seed, workerPass string, oidc oidcConfig, admitAnon string) string {
	u := url.URL{
		Scheme: "nats",
		User:   url.UserPassword(workerUser, workerPass),
		Host:   fmt.Sprintf("127.0.0.1:%d", natsCorePort),
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.\n")
	fmt.Fprint(b, resolveKernelPins())
	fmt.Fprintf(b, "pgck.nats_account_seed = '%s'\n", sqlQuote(seed))
	fmt.Fprintf(b, "pgck.nats_url = '%s'\n", sqlQuote(u.String()))
	// Omitted entirely when the document could not be resolved — never a URL and
	// never a partial value. An absent GUC degrades to the anonymous tier; a
	// present-but-unusable one would combine with strict admittance to refuse
	// every connection on the instance.
	if oidc.jwks != "" {
		fmt.Fprintf(b, "pgck.oidc_jwks = '%s'\n", sqlQuote(oidc.jwks))
	}
	fmt.Fprintf(b, "pgck.oidc_issuer = '%s'\n", sqlQuote(oidc.issuer))
	fmt.Fprintf(b, "pgck.oidc_audience = '%s'\n", sqlQuote(oidc.audience))
	// THE SEVENTH KEY. A door that verifies tokens must not also admit
	// unverified ones: with this absent, pgCK's default `true` silently
	// DOWNGRADES a token that fails verification (wrong audience, expired,
	// foreign realm) to the anonymous tier, and the client sees an open socket
	// and no error. Emitted here so `source` is `configuration file`, not
	// `default` — an unowned value is one volume wipe from reverting.
	fmt.Fprintf(b, "pgck.admit_anonymous = %s\n", admitAnon)
	return b.String()
}

// anonPgckConf renders the no-realm fragment: an anonymous nats_url and NO
// account seed, so pgCK's callout responder is not started.
func anonPgckConf(admitAnon string) string {
	// Explicit `on`, not merely pgCK's default: this is the DECLARED anonymous
	// tier, and declaring it is what makes the realm branch's `off` meaningful.
	// A bare `docker run` is correct to be anonymous — with no realm there is no
	// other coherent posture — but it should say so in a file, not by omission.
	return fmt.Sprintf(`# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.
%spgck.nats_url = 'nats://127.0.0.1:%d'
pgck.admit_anonymous = %s
`, resolveKernelPins(), natsCorePort, admitAnon)
}

const defaultProject = "demo"

// canonicalKernel is the form ckp.germinate_kernel enforces: lowercase, dashes
// optional, ONE transport segment. A wrong-case or dotted segment matches
// nothing on the wire and is indistinguishable from never having been added —
// which is why pgCK 0.4.77 refuses even its own former default 'pgCK', and why
// nothing here may rely on a default.
var canonicalKernel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// kernelPins writes the two planes that must agree, in the one place that
// writes them:
//
//	ckp.project   — which kernel a seal LANDS IN (the SQL/dispatch plane); one value
//	pgck.kernels  — which kernels the wire SERVES (the NATS subject plane); a SET.
//	                The auth-callout mints permissions per listed kernel as
//	                input.kernel.<K>.id.<sub>.action.>, event.kernel.<K>.> and
//	                result.kernel.<K>.>
//
// The invariant is MEMBERSHIP, not equality: the project must be one of the
// served kernels. When it was not (ckp.project falling back to 'demo' while the
// wire served 'pgck'), a verified client dispatched into a kernel carrying a
// bare core surface and the seal was refused "type not admitted", while the
// adopted surface sat elsewhere emitting events nobody held a grant to hear.
// Measured 2026-08-20; it burned ck-allinone v0.7.33.
//
// BOTH COME FROM CONFIG. A hardcoded single kernel meant germinating a new one
// left it UNREACHABLE until an operator hand-edited a GUC — pgCK hit this twice
// in one day (SuperAiHarness3000, then their own door reporting NOT ADMITTED),
// and asked that the kernel set travel with ckp.project. So:
//
//	OCIGER_CK_PROJECT  — the seal plane; defaults to "demo"
//	OCIGER_CK_KERNELS  — comma-separated set the wire serves; defaults to the project
//
// The project is ALWAYS a member: it is prepended when an operator names a set
// without it. That keeps the invariant true by construction rather than by an
// operator remembering it — the failure it prevents is silent, not loud.
func kernelPins(project, kernels string) (string, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		project = defaultProject
	}
	if !canonicalKernel.MatchString(project) {
		return "", fmt.Errorf("OCIGER_CK_PROJECT %q is not canonical (lowercase, dashes optional, one segment)", project)
	}

	set := []string{project}
	seen := map[string]bool{project: true}
	for _, k := range strings.Split(kernels, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if !canonicalKernel.MatchString(k) {
			return "", fmt.Errorf("OCIGER_CK_KERNELS entry %q is not canonical (lowercase, dashes optional, one segment)", k)
		}
		if !seen[k] {
			set = append(set, k)
			seen[k] = true
		}
	}

	return fmt.Sprintf("ckp.project = '%s'\npgck.kernels = '%s'\n",
		sqlQuote(project), sqlQuote(strings.Join(set, ","))), nil
}

// resolveKernelPins reads the two settings from the environment and fails LOUDLY
// on a bad value. A misconfigured kernel name must not degrade to a default:
// that is how a bundle ends up serving a kernel nobody meant and refusing the
// one they did.
func resolveKernelPins() string {
	pins, err := kernelPins(os.Getenv("OCIGER_CK_PROJECT"), os.Getenv("OCIGER_CK_KERNELS"))
	if err != nil {
		log.Fatalf("ociger-ck-identity: %v", err)
	}
	return pins
}

func sqlQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func writeFileMode(path, contents string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(contents), mode)
}

// writePgckConf writes the pgck secrets fragment 0640 and chowns it to postgres
// (999) — the sole reader. The chown is mandatory: postgres reads the file via
// include_if_exists as uid 999, so a root-owned 0640 file would be unreadable to
// it (→ include skipped → no seed: the original v0.7.30 boot bug). This keeps the
// account seed out of reach of the dropped-to-non-root nats/httpd services.
func writePgckConf(path, contents string) error {
	if err := writeFileMode(path, contents, 0o640); err != nil {
		return err
	}
	return os.Chown(path, postgresUID, postgresGID)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
