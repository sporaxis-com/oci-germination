// ociger-pgck-relay — NATS ↔ pgCK dispatch bridge.
//
// Subscribes to `input.kernel.<K>.action.<verb>` (NATS), connects to
// Postgres as ck_participant via the Unix socket, calls
// `SELECT ckp.dispatch(verb, kernel_urn, payload::jsonb, identity)`,
// and publishes the typed jsonb reply on `result.kernel.<K>.<verb>`
// preserving the inbound headers (including Trace-Id).
//
// History
// -------
// v0.7.5–v0.7.10 — bus-level fan-out only: forwarded the inbound MSG
//   verbatim to `event.kernel.<K>.<verb>` without ever calling
//   `ckp.dispatch`. CK.Lib.Js verify-v150 exercised this path and
//   correctly reported that nothing reached `result.*` — the relay
//   echoed but did not dispatch.
//
// v0.7.11 — replaces the echo with a real bridge per CK.Lib.Js NOTIFY
//   `…v0.7.9.integration-gaps-block-v150.md` ask #2: subscribe input,
//   CALL ckp.dispatch, publish typed result on `result.kernel.<K>.<verb>`.
//
// Self-disable conditions (unchanged from v0.7.10):
//
//   1. env OCIGER_DISABLE_PGCK_RELAY=1 → block forever, never connect.
//   2. boot probe: if /usr/lib/postgresql/17/lib/pgck.so contains BOTH
//      "RELAY_OUT_PREFIX" AND "async_nats::" — the discriminating
//      conjunction of the relay-code constant and a symbol from the
//      async_nats Rust crate — we assume the upstream `nats-client`
//      Cargo feature is compiled in and stand down.
//
// Standing down means "park as a no-op longrun" rather than exit so
// s6 doesn't restart-loop us.
//
// Postgres connection
// -------------------
// The relay connects to postgres at 127.0.0.1:5432 as `ck_participant`.
// The launcher's pg_hba puts `host all ck_participant 0.0.0.0/0
// scram-sha-256` BEFORE the catch-all `host all all all trust`, so
// when OCIGER_CK_PARTICIPANT_PASSWORD is set (which the bundle requires
// at deploy), the relay must authenticate too. We read the same env
// the launcher reads and pass it as the connection password. If the
// env is unset, ck_participant has no password and cannot log in —
// the relay will retry forever (matches the "refuse-on-missing"
// posture documented in init.sql).
//
// ck_participant has EXECUTE on `ckp.dispatch` via the SECURITY DEFINER
// grant that CREATE EXTENSION pgck CASCADE applies.
//
// Identity is passed through the dispatch call as the 4th argument.
// For now it reads from the inbound NATS header `X-Identity-Key`; the
// Envoy/Keycloak edge sets that header when present. If absent we pass
// the empty string and pgCK's TR-02 transitional GUC handles fallback.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

const (
	relayInSubject = "input.kernel.*.action.>"
	relayOutPrefix = "result.kernel."
	pgckSoPath     = "/usr/lib/postgresql/17/lib/pgck.so"
	// Discriminating multi-marker conjunction (ratified with pgCK 2026-06-10):
	// either alone can appear in unrelated source/SQL text; the BOTH-present
	// case is the upstream nats-client feature build.
	probeMarkerRelay = "RELAY_OUT_PREFIX"
	probeMarkerAsync = "async_nats::"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("pgck-relay: ")

	if os.Getenv("OCIGER_DISABLE_PGCK_RELAY") == "1" {
		log.Println("OCIGER_DISABLE_PGCK_RELAY=1 — standing down (will not connect)")
		park()
	}
	if pgckHasRelayCode(pgckSoPath) {
		log.Printf("%s contains both %q and %q — upstream nats-client build is live; standing down", pgckSoPath, probeMarkerRelay, probeMarkerAsync)
		park()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Postgres pool — TCP + scram-sha-256 (matches pg_hba ordering set by
	// the launcher when OCIGER_CK_PARTICIPANT_HBA=1).
	pgPassword := os.Getenv("OCIGER_CK_PARTICIPANT_PASSWORD")
	pgURL := os.Getenv("OCIGER_PGCK_RELAY_PG_URL")
	if pgURL == "" {
		pgURL = fmt.Sprintf(
			"postgres://ck_participant:%s@127.0.0.1:5432/postgres?sslmode=disable",
			url.QueryEscape(pgPassword),
		)
	}
	pool := mustOpenPg(ctx, pgURL)
	defer pool.Close()

	// NATS connection.
	natsURL := getenv("OCIGER_PGCK_RELAY_URL", "nats://127.0.0.1:4222")
	log.Printf("connecting NATS %s", natsURL)
	nc, err := nats.Connect(natsURL,
		nats.Name("ociger-pgck-relay"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, e error) {
			if e != nil {
				log.Printf("NATS disconnected: %v", e)
			}
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			log.Printf("NATS reconnected to %s", c.ConnectedUrl())
		}),
	)
	if err != nil {
		log.Fatalf("NATS connect failed: %v", err)
	}
	defer nc.Drain()

	sub, err := nc.Subscribe(relayInSubject, func(m *nats.Msg) {
		handleMsg(ctx, pool, nc, m)
	})
	if err != nil {
		log.Fatalf("subscribe %s failed: %v", relayInSubject, err)
	}
	defer sub.Unsubscribe()

	log.Printf("dispatching %s → %s<K>.<verb> via ckp.dispatch", relayInSubject, relayOutPrefix)
	<-ctx.Done()
	log.Println("shutting down")
}

// handleMsg parses the inbound subject, calls ckp.dispatch, and
// publishes the typed reply.
//
// pgCK 0.3.x..0.4.x ships TWO `ckp.dispatch` overloads:
//
//   ckp.dispatch(verb text, payload jsonb)                                 — the GOVERNED 2-arg
//   ckp.dispatch(verb text, kernel_urn text, payload jsonb, identity text) — the CI-A-2 STUB
//
// The 2-arg form runs the per-verb seal handlers and returns a sealed
// instance + proof_digest on success. The 4-arg form is a CI-A-2 stub
// that returns `{"ok":false,"error":"verb not governed yet (CI-B)",…}`
// for every verb — useful for the role-floor probe, useless for a real
// relay. v0.7.11..v0.7.13 called the 4-arg form by mistake, which
// CK.Lib.Js verify-v150 caught with the not-governed reply. v0.7.14+
// calls the 2-arg form — the one that actually seals.
//
// We retain the kernelName + identity locally for trace context but they
// no longer go into the dispatch call. pgCK reads project/identity from
// session GUCs (when seal-handlers want them); when they're absent the
// 2-arg path still seals against the default kernel substrate.
func handleMsg(ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn, m *nats.Msg) {
	// Subject: input.kernel.<K>.action.<verb…>
	// Split off the first 4 tokens; the rest is the verb (which may
	// contain dots, e.g. `task.create`, `instance.transition`).
	parts := strings.SplitN(m.Subject, ".", 5)
	if len(parts) < 5 || parts[0] != "input" || parts[1] != "kernel" || parts[3] != "action" {
		log.Printf("skip malformed subject %q", m.Subject)
		return
	}
	kernelName := parts[2]
	verb := parts[4]

	// payload defaults to '{}' if the inbound is empty so the jsonb cast holds.
	payload := string(m.Data)
	if strings.TrimSpace(payload) == "" {
		payload = "{}"
	}

	// Call the GOVERNED 2-arg dispatch — `SELECT ckp.dispatch($verb::text,
	// $payload::jsonb)::text`. pgCK seals against the payload's own
	// kernel/target_kernel field per the per-verb handlers in
	// pgck--0.4.x.sql.
	var resultBytes []byte
	row := pool.QueryRow(ctx,
		"SELECT ckp.dispatch($1::text, $2::jsonb)::text",
		verb, payload)
	if err := row.Scan(&resultBytes); err != nil {
		log.Printf("dispatch %s on %s failed: %v", verb, kernelName, err)
		resultBytes = encodeError(err.Error())
	}

	// Publish on result.kernel.<K>.<verb>, preserving inbound headers.
	out := relayOutPrefix + kernelName + "." + verb
	fwd := &nats.Msg{Subject: out, Data: resultBytes, Header: m.Header}
	if err := nc.PublishMsg(fwd); err != nil {
		log.Printf("publish %s failed: %v", out, err)
	}
}

func encodeError(reason string) []byte {
	envelope := map[string]any{
		"ok":    false,
		"error": fmt.Sprintf("relay dispatch failed: %s", reason),
	}
	b, _ := json.Marshal(envelope)
	return b
}

func mustOpenPg(ctx context.Context, url string) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatalf("pg url parse failed: %v", err)
	}
	cfg.MaxConns = 4
	cfg.MinConns = 1
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = "ociger-pgck-relay"

	// Retry until pg is ready (postgres may still be starting under s6).
	deadline := time.Now().Add(60 * time.Second)
	for {
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Printf("connected to %s", redactPassword(url))
				return pool
			} else {
				pool.Close()
				err = pingErr
			}
		}
		if time.Now().After(deadline) {
			log.Fatalf("pg connect failed after 60s: %v", err)
		}
		log.Printf("pg connect retry: %v", err)
		select {
		case <-ctx.Done():
			log.Fatalf("pg connect aborted: %v", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func pgckHasRelayCode(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(probeMarkerRelay)) &&
		bytes.Contains(data, []byte(probeMarkerAsync))
}

func park() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
	log.Println("park exit")
	os.Exit(0)
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// redactPassword replaces the userinfo password component of a pg URL
// with "***" so connect logs don't leak the secret to the container's
// stdout (which s6 will surface to `docker logs`).
func redactPassword(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	if _, hasPw := u.User.Password(); hasPw {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}
