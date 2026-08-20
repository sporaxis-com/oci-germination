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

	if !realm {
		if err := writeFileMode(natsConf, anonNatsServerConf(), 0o644); err != nil {
			log.Fatalf("ociger-ck-identity: write %s: %v", natsConf, err)
		}
		if err := writePgckConf(pgckConf, anonPgckConf()); err != nil {
			log.Fatalf("ociger-ck-identity: write %s: %v", pgckConf, err)
		}
		log.Printf("ociger-ck-identity: no OIDC realm (OCIGER_OIDC_*) → ANONYMOUS mode (callout off); wrote %s + %s", natsConf, pgckConf)
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
	if err := writePgckConf(pgckConf, pgckSecretsConf(seed, workerPass, oidc)); err != nil {
		log.Fatalf("ociger-ck-identity: write %s: %v", pgckConf, err)
	}

	source := "minted per-boot (ephemeral)"
	if strings.TrimSpace(os.Getenv("OCIGER_NATS_ACCOUNT_SEED")) != "" {
		source = "operator-supplied"
	}
	verify := "OFF (anonymous tier) — pgck.oidc_jwks omitted"
	if oidc.jwks != "" {
		verify = fmt.Sprintf("ON — JWKS document from %s (%d bytes, never fetched)", oidc.jwksSource, len(oidc.jwks))
	}
	log.Printf("ociger-ck-identity: OIDC realm %q → auth-callout ACTIVE; account %s [%s]; token verify: %s; wrote %s + %s",
		oidc.issuer, pub, source, verify, natsConf, pgckConf)
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
func pgckSecretsConf(seed, workerPass string, oidc oidcConfig) string {
	u := url.URL{
		Scheme: "nats",
		User:   url.UserPassword(workerUser, workerPass),
		Host:   fmt.Sprintf("127.0.0.1:%d", natsCorePort),
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.\n")
	fmt.Fprint(b, kernelPins)
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
	return b.String()
}

// anonPgckConf renders the no-realm fragment: an anonymous nats_url and NO
// account seed, so pgCK's callout responder is not started.
func anonPgckConf() string {
	return fmt.Sprintf(`# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.
%spgck.nats_url = 'nats://127.0.0.1:%d'
`, kernelPins, natsCorePort)
}

// kernelPins names THE kernel this bundle serves, on both planes that must
// agree, in the one place that writes them.
//
//	ckp.project   — which kernel a seal lands in (the SQL/dispatch plane)
//	pgck.kernels  — which kernel the wire serves (the NATS subject plane):
//	                the auth-callout mints permissions per listed kernel as
//	                input.kernel.<K>.id.<sub>.action.>, event.kernel.<K>.>,
//	                result.kernel.<K>.>
//
// They MUST be the same string. When they diverged (ckp.project falling back
// to 'demo' while the wire served 'pgck'), a verified client dispatched into
// kernel 'pgck' — a DIFFERENT kernel carrying a bare core surface — and the
// seal was refused "type wave#Finding is not admitted", while the adopted
// surface sat in 'demo' and its sealed events rode event.kernel.demo.> where
// no client had permission to hear them. Both halves measured 2026-08-20.
//
// Pinned, not inherited: 'demo' is ckp._project()'s FALLBACK, and pgCK's own
// note calls that fallback "a landing site for writes that belong to nobody"
// and intends to make it fail-closed. Writing it explicitly keeps this bundle
// working through that change. Lowercase, one transport segment — the
// canonical form ckp.germinate_kernel enforces; 0.4.77 refuses pgCK's own
// compiled-in default 'pgCK' as non-canonical, which is why nothing may rely
// on that default.
const kernelPins = "ckp.project = 'demo'\npgck.kernels = 'demo'\n"

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
