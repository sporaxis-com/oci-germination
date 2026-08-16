package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/nkeys"
)

// A minted account must yield a public key that round-trips through its seed —
// this is the invariant that keeps the broker issuer and pgCK's signing seed in
// agreement.
func TestResolveAccount_MintsRoundTrippingAccount(t *testing.T) {
	seed, pub, err := resolveAccount("")
	if err != nil {
		t.Fatalf("resolveAccount(\"\"): %v", err)
	}
	if !strings.HasPrefix(pub, "A") {
		t.Fatalf("minted public key %q is not an account nkey (want 'A' prefix)", pub)
	}
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		t.Fatalf("returned seed does not parse: %v", err)
	}
	got, err := kp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if got != pub {
		t.Fatalf("seed→pub %q != reported pub %q", got, pub)
	}
}

// Two mints must differ — proving the account is freshly generated, not a baked
// constant.
func TestResolveAccount_MintsAreUnique(t *testing.T) {
	_, pub1, err := resolveAccount("")
	if err != nil {
		t.Fatal(err)
	}
	_, pub2, err := resolveAccount("")
	if err != nil {
		t.Fatal(err)
	}
	if pub1 == pub2 {
		t.Fatalf("two mints produced the same account %q — not random", pub1)
	}
}

// An operator-supplied account seed is honored verbatim (stable identity).
func TestResolveAccount_HonorsOperatorSeed(t *testing.T) {
	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	sb, _ := kp.Seed()
	wantPub, _ := kp.PublicKey()

	seed, pub, err := resolveAccount(string(sb))
	if err != nil {
		t.Fatalf("resolveAccount(operator seed): %v", err)
	}
	if pub != wantPub {
		t.Fatalf("operator seed pub %q != %q", pub, wantPub)
	}
	if seed != string(sb) {
		t.Fatalf("operator seed not returned verbatim")
	}
}

// A non-account seed (here a USER seed) must be rejected — using it as the
// callout issuer would make the broker reject every minted admittance.
func TestResolveAccount_RejectsNonAccountSeed(t *testing.T) {
	kp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	sb, _ := kp.Seed()
	if _, _, err := resolveAccount(string(sb)); err == nil {
		t.Fatal("expected a user seed to be rejected as a non-account nkey")
	}
}

func TestResolveAccount_RejectsGarbage(t *testing.T) {
	if _, _, err := resolveAccount("not-a-real-seed"); err == nil {
		t.Fatal("expected garbage seed to error")
	}
}

func TestResolveWorkerPassword(t *testing.T) {
	if got, _ := resolveWorkerPassword("hunter2"); got != "hunter2" {
		t.Fatalf("operator password not honored: %q", got)
	}
	p, err := resolveWorkerPassword("")
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 48 { // 24 random bytes → 48 hex chars
		t.Fatalf("generated password length = %d, want 48", len(p))
	}
	p2, _ := resolveWorkerPassword("")
	if p == p2 {
		t.Fatal("two generated passwords are identical — not random")
	}
}

var testOIDC = oidcConfig{jwks: `{"keys":[{"kty":"OKP"}]}`, issuer: "https://realm.test/", audience: "ck-allinone"}

// REALM-mode configs must carry the callout issuer on the nats side and the
// seed + worker-cred URL + oidc verifier config on the pg side.
func TestRenderedConfigs(t *testing.T) {
	nats := natsServerConf("ATESTPUBKEY", "wpass")
	for _, want := range []string{"auth_callout", "issuer: \"ATESTPUBKEY\"", "auth_users: [ pgck_worker ]", "port: 9222"} {
		if !strings.Contains(nats, want) {
			t.Fatalf("nats-server.conf missing %q:\n%s", want, nats)
		}
	}
	if strings.Contains(nats, "no_auth_user") {
		t.Fatal("REALM nats-server.conf must NOT have no_auth_user (callout admits everyone)")
	}

	pg := pgckSecretsConf("SATESTSEED", "wpass", testOIDC)
	for _, want := range []string{
		"pgck.nats_account_seed = 'SATESTSEED'",
		"nats://pgck_worker:wpass@127.0.0.1:4222",
		"pgck.oidc_issuer = 'https://realm.test/'",
		"pgck.oidc_audience = 'ck-allinone'",
		`pgck.oidc_jwks = '{"keys":[{"kty":"OKP"}]}'`,
	} {
		if !strings.Contains(pg, want) {
			t.Fatalf("pgck.conf missing %q:\n%s", want, pg)
		}
	}
}

// A worker password with URL-reserved characters must stay well-formed in the
// nats_url (operators may supply their own).
func TestPgckSecretsConf_EncodesReservedPassword(t *testing.T) {
	pg := pgckSecretsConf("SATESTSEED", "p@ss:w/rd", testOIDC)
	if strings.Contains(pg, "p@ss:w/rd") {
		t.Fatalf("reserved chars not encoded in nats_url:\n%s", pg)
	}
	if !strings.Contains(pg, "pgck_worker:p%40ss%3Aw%2Frd@127.0.0.1:4222") {
		t.Fatalf("expected percent-encoded userinfo:\n%s", pg)
	}
}

// A JWKS containing a single quote must be doubled for postgresql.conf.
func TestPgckSecretsConf_EscapesSingleQuote(t *testing.T) {
	pg := pgckSecretsConf("S", "w", oidcConfig{jwks: `a'b`, issuer: "i", audience: "a"})
	if !strings.Contains(pg, "pgck.oidc_jwks = 'a''b'") {
		t.Fatalf("single quote not doubled:\n%s", pg)
	}
}

// resolveOIDC requires all three fields; a partial config is anonymous mode.
func TestResolveOIDC(t *testing.T) {
	t.Setenv("OCIGER_OIDC_JWKS", "")
	t.Setenv("OCIGER_OIDC_ISSUER", "")
	t.Setenv("OCIGER_OIDC_AUDIENCE", "")
	if _, ok := resolveOIDC(); ok {
		t.Fatal("empty env must be anonymous mode")
	}
	t.Setenv("OCIGER_OIDC_JWKS", "{}")
	t.Setenv("OCIGER_OIDC_ISSUER", "iss")
	if _, ok := resolveOIDC(); ok {
		t.Fatal("partial config (no audience) must be anonymous mode")
	}
	t.Setenv("OCIGER_OIDC_AUDIENCE", "aud")
	c, ok := resolveOIDC()
	if !ok || c.issuer != "iss" || c.audience != "aud" {
		t.Fatalf("full config should be realm mode: ok=%v c=%+v", ok, c)
	}
}

// Anonymous-mode configs: no callout / no account seed, so anon can dispatch.
func TestAnonModeConfigs(t *testing.T) {
	nats := anonNatsServerConf()
	if !strings.Contains(nats, "no_auth_user: anon") {
		t.Fatalf("anon nats-server.conf must map anon via no_auth_user:\n%s", nats)
	}
	if strings.Contains(nats, "auth_callout") {
		t.Fatalf("anon nats-server.conf must NOT have auth_callout:\n%s", nats)
	}
	pg := anonPgckConf()
	if strings.Contains(pg, "nats_account_seed") {
		t.Fatalf("anon pgck.conf must NOT set an account seed (responder stays off):\n%s", pg)
	}
	if !strings.Contains(pg, "pgck.nats_url = 'nats://127.0.0.1:4222'") {
		t.Fatalf("anon pgck.conf must set an anonymous nats_url:\n%s", pg)
	}
}

// ── og#29: the JWKS is a DOCUMENT, injected at start, never fetched ──────────

const goodJWKS = `{"keys":[{"kty":"OKP","crv":"Ed25519","kid":"k1","x":"AAAA"}]}`

func TestValidateJWKS_RefusesURL(t *testing.T) {
	for _, u := range []string{
		"https://id.example.test/realms/R/protocol/openid-connect/certs",
		"http://id.example.test/certs",
	} {
		if _, err := validateJWKS(u); err == nil {
			t.Fatalf("a URL MUST be refused, got nil error for %q", u)
		} else if !strings.Contains(err.Error(), "URL") {
			t.Fatalf("the refusal must name the URL mistake, got: %v", err)
		}
	}
}

func TestValidateJWKS_RefusesNonJWKS(t *testing.T) {
	for name, in := range map[string]string{
		"empty":        "",
		"not json":     "hello",
		"json no keys": `{"issuer":"x"}`,
		"empty keys":   `{"keys":[]}`,
	} {
		if _, err := validateJWKS(in); err == nil {
			t.Fatalf("%s MUST be refused", name)
		}
	}
}

func TestValidateJWKS_AcceptsAndCompacts(t *testing.T) {
	got, err := validateJWKS("  {\n \"keys\" : [ {\"kty\":\"OKP\",\"crv\":\"Ed25519\"} ]\n}  ")
	if err != nil {
		t.Fatalf("a well-formed JWKS must be accepted: %v", err)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Fatalf("the value must be one line (postgresql.conf), got %q", got)
	}
}

// A realm may legitimately publish several key types — the bench realm ships
// EdDSA + RS256 + RSA-OAEP — so ONE usable key is enough and the rest are not
// our business to reject.
func TestValidateJWKS_AcceptsMixedSetWithOneEd25519(t *testing.T) {
	mixed := `{"keys":[{"kty":"RSA","n":"x","e":"AQAB"},{"kty":"OKP","crv":"Ed25519","x":"abc"},{"kty":"RSA","alg":"RSA-OAEP"}]}`
	if _, err := validateJWKS(mixed); err != nil {
		t.Fatalf("a mixed JWKS carrying one Ed25519 key must be accepted: %v", err)
	}
}

// The negative control this gate exists for. Without it an RSA-only document
// is accepted, pgck.oidc_jwks is written, this process logs "token verify: ON",
// and pgCK — whose verifier is EdDSA-only — loads no key and admits everyone
// anonymously. Two layers reporting healthy, nobody verified.
func TestValidateJWKS_RefusesRSAOnlyRealm(t *testing.T) {
	for name, in := range map[string]string{
		// Shaped like an Azure Entra key set: well-formed, ≥1 key, all RS256.
		"entra-style RSA only": `{"keys":[{"kty":"RSA","use":"sig","alg":"RS256","kid":"a","n":"x","e":"AQAB"},{"kty":"RSA","use":"sig","alg":"RS256","kid":"b","n":"y","e":"AQAB"}]}`,
		"EC only":              `{"keys":[{"kty":"EC","crv":"P-256","x":"a","y":"b"}]}`,
		// OKP but the wrong curve — pgCK keeps only crv:Ed25519.
		"OKP wrong curve": `{"keys":[{"kty":"OKP","crv":"X25519","x":"a"}]}`,
	} {
		if _, err := validateJWKS(in); err == nil {
			t.Fatalf("%s MUST be refused: pgCK would load no key and silently degrade to anonymous", name)
		}
	}
}

func TestResolveJWKS_FileWinsOverEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "jwks.json")
	if err := os.WriteFile(p, []byte(goodJWKS), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCIGER_OIDC_JWKS_FILE", p)
	t.Setenv("OCIGER_OIDC_JWKS", `{"keys":[{"kty":"IGNORED"}]}`)

	doc, src, err := resolveJWKS()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc, "Ed25519") {
		t.Fatalf("the FILE must win over the env value, got %q", doc)
	}
	if !strings.Contains(src, p) {
		t.Fatalf("the source must name the file, got %q", src)
	}
}

func TestResolveJWKS_MissingFileIsRefusedNotSilent(t *testing.T) {
	t.Setenv("OCIGER_OIDC_JWKS_FILE", "/nonexistent/jwks.json")
	t.Setenv("OCIGER_OIDC_JWKS", "")
	if _, _, err := resolveJWKS(); err == nil {
		t.Fatal("an unreadable JWKS file MUST be an error, never a silent skip")
	}
}

// The failure ladder: an unusable JWKS omits the GUC entirely — never a URL,
// never a partial value — while the realm stays ACTIVE so the responder starts
// and pgCK degrades to the anonymous tier.
func TestPgckSecretsConf_OmitsJWKSWhenUnusable(t *testing.T) {
	out := pgckSecretsConf("SEED", "pw", oidcConfig{issuer: "iss", audience: "aud"})
	if strings.Contains(out, "pgck.oidc_jwks") {
		t.Fatalf("an unusable JWKS MUST omit the GUC entirely, got:\n%s", out)
	}
	for _, want := range []string{"pgck.oidc_issuer", "pgck.oidc_audience", "pgck.nats_account_seed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("omitting the JWKS must not drop %s:\n%s", want, out)
		}
	}
}

func TestPgckSecretsConf_WritesJWKSDocumentWhenUsable(t *testing.T) {
	out := pgckSecretsConf("SEED", "pw", oidcConfig{jwks: goodJWKS, issuer: "iss", audience: "aud"})
	if !strings.Contains(out, "pgck.oidc_jwks = '"+goodJWKS+"'") {
		t.Fatalf("the document must be written verbatim, got:\n%s", out)
	}
	// header + seed + nats_url + jwks + issuer + audience
	if got := strings.Count(out, "\n"); got != 6 {
		t.Fatalf("expected 6 lines (header + 5 settings), got %d:\n%s", got, out)
	}
}

// A URL keeps the bundle in REALM mode (responder up, verify off) rather than
// dropping to ANONYMOUS mode, which would also disable the responder.
func TestResolveOIDC_URLStaysRealmButOmitsJWKS(t *testing.T) {
	t.Setenv("OCIGER_OIDC_JWKS_FILE", "")
	t.Setenv("OCIGER_OIDC_JWKS", "https://id.example.test/certs")
	t.Setenv("OCIGER_OIDC_ISSUER", "https://id.example.test/realms/R")
	t.Setenv("OCIGER_OIDC_AUDIENCE", "aud")

	c, realm := resolveOIDC()
	if !realm {
		t.Fatal("a URL must NOT drop the bundle out of realm mode")
	}
	if c.jwks != "" {
		t.Fatalf("a URL must resolve to an EMPTY document, got %q", c.jwks)
	}
}

// The contract, asserted rather than assumed: this command performs no egress.
func TestNoEgressInThisPackage(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{`"net/http"`, "http.Get(", "http.Client{"} {
		if strings.Contains(string(src), banned) {
			t.Fatalf("ociger-ck-identity MUST NOT perform egress; found %s", banned)
		}
	}
}
