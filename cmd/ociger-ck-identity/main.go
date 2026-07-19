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
	"crypto/rand"
	"encoding/hex"
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
)

// oidcConfig is the operator-supplied realm the callout verifies bearers against.
type oidcConfig struct {
	jwks     string
	issuer   string
	audience string
}

func main() {
	natsConf := getenv("OCIGER_NATS_CONF", defaultNatsConf)
	pgckConf := getenv("OCIGER_PGCK_CONF", defaultPgckConf)

	oidc, realm := resolveOIDC()

	if !realm {
		if err := writeFileMode(natsConf, anonNatsServerConf(), 0o644); err != nil {
			log.Fatalf("ociger-ck-identity: write %s: %v", natsConf, err)
		}
		if err := writeFileMode(pgckConf, anonPgckConf(), 0o644); err != nil {
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

	// Both configs are 0644: postgres reads pgck.conf via include_if_exists as
	// its own uid (999), which is not root and not in root's group — a 0640
	// root-owned file is unreadable to it and postgres silently treats the
	// include as "missing". /run is not externally reachable and the container is
	// single-tenant, so world-readable inside the container is acceptable (the
	// same posture as nats-server.conf, which already carries the worker password).
	if err := writeFileMode(natsConf, natsServerConf(pub, workerPass), 0o644); err != nil {
		log.Fatalf("ociger-ck-identity: write %s: %v", natsConf, err)
	}
	if err := writeFileMode(pgckConf, pgckSecretsConf(seed, workerPass, oidc), 0o644); err != nil {
		log.Fatalf("ociger-ck-identity: write %s: %v", pgckConf, err)
	}

	source := "minted per-boot (ephemeral)"
	if strings.TrimSpace(os.Getenv("OCIGER_NATS_ACCOUNT_SEED")) != "" {
		source = "operator-supplied"
	}
	log.Printf("ociger-ck-identity: OIDC realm %q → auth-callout ACTIVE; account %s [%s]; wrote %s + %s", oidc.issuer, pub, source, natsConf, pgckConf)
}

// resolveOIDC reads the realm config. All three fields are required — a partial
// config is a misconfiguration, treated as "no realm" (anonymous mode).
func resolveOIDC() (oidcConfig, bool) {
	c := oidcConfig{
		jwks:     strings.TrimSpace(os.Getenv("OCIGER_OIDC_JWKS")),
		issuer:   strings.TrimSpace(os.Getenv("OCIGER_OIDC_ISSUER")),
		audience: strings.TrimSpace(os.Getenv("OCIGER_OIDC_AUDIENCE")),
	}
	return c, c.jwks != "" && c.issuer != "" && c.audience != ""
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
      permissions: { publish: { deny: "input.kernel.pgCK.action.*.sealed" } } }
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
	return fmt.Sprintf(`# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.
pgck.nats_account_seed = '%s'
pgck.nats_url = '%s'
pgck.oidc_jwks = '%s'
pgck.oidc_issuer = '%s'
pgck.oidc_audience = '%s'
`, sqlQuote(seed), sqlQuote(u.String()), sqlQuote(oidc.jwks), sqlQuote(oidc.issuer), sqlQuote(oidc.audience))
}

// anonPgckConf renders the no-realm fragment: an anonymous nats_url and NO
// account seed, so pgCK's callout responder is not started.
func anonPgckConf() string {
	return fmt.Sprintf(`# GENERATED by ociger-ck-identity at boot — DO NOT EDIT.
pgck.nats_url = 'nats://127.0.0.1:%d'
`, natsCorePort)
}

func sqlQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func writeFileMode(path, contents string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(contents), mode)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
