package main

import (
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

// The rendered configs must carry the issuer + callout on the nats side and the
// seed + worker-cred URL on the pg side, so the two halves reference one account.
func TestRenderedConfigs(t *testing.T) {
	nats := natsServerConf("ATESTPUBKEY", "wpass")
	for _, want := range []string{"auth_callout", "issuer: \"ATESTPUBKEY\"", "auth_users: [ pgck_worker ]", "port: 9222"} {
		if !strings.Contains(nats, want) {
			t.Fatalf("nats-server.conf missing %q:\n%s", want, nats)
		}
	}
	if strings.Contains(nats, "no_auth_user") {
		t.Fatal("nats-server.conf still has no_auth_user — the interim forge-deny must be gone under the callout")
	}

	pg := pgckSecretsConf("SATESTSEED", "wpass")
	for _, want := range []string{"pgck.nats_account_seed = 'SATESTSEED'", "nats://pgck_worker:wpass@127.0.0.1:4222"} {
		if !strings.Contains(pg, want) {
			t.Fatalf("pgck.conf missing %q:\n%s", want, pg)
		}
	}
}

// A worker password with URL-reserved characters must stay well-formed in the
// nats_url (operators may supply their own).
func TestPgckSecretsConf_EncodesReservedPassword(t *testing.T) {
	pg := pgckSecretsConf("SATESTSEED", "p@ss:w/rd")
	if strings.Contains(pg, "p@ss:w/rd") {
		t.Fatalf("reserved chars not encoded in nats_url:\n%s", pg)
	}
	if !strings.Contains(pg, "pgck_worker:p%40ss%3Aw%2Frd@127.0.0.1:4222") {
		t.Fatalf("expected percent-encoded userinfo:\n%s", pg)
	}
}
