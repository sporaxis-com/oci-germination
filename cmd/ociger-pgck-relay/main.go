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
//
//	verbatim to `event.kernel.<K>.<verb>` without ever calling
//	`ckp.dispatch`. CK.Lib.Js verify-v150 exercised this path and
//	correctly reported that nothing reached `result.*` — the relay
//	echoed but did not dispatch.
//
// v0.7.11 — replaces the echo with a real bridge per CK.Lib.Js NOTIFY
//
//	`…v0.7.9.integration-gaps-block-v150.md` ask #2: subscribe input,
//	CALL ckp.dispatch, publish typed result on `result.kernel.<K>.<verb>`.
//
// Self-disable conditions (unchanged from v0.7.10):
//
//  1. env OCIGER_DISABLE_PGCK_RELAY=1 → block forever, never connect.
//  2. boot probe: if /usr/lib/postgresql/17/lib/pgck.so contains BOTH
//     "RELAY_OUT_PREFIX" AND "async_nats::" — the discriminating
//     conjunction of the relay-code constant and a symbol from the
//     async_nats Rust crate — we assume the upstream `nats-client`
//     Cargo feature is compiled in and stand down.
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
//
// Outbox drain (v0.7.18+)
// -----------------------
// Every pgCK seal enqueues one row into `ckp.outbox` (subject, payload,
// headers) via the ckp.ledger_to_outbox trigger. pgCK's design drains
// that queue with an in-kernel nats-client bgworker — which is NOT in the
// shipped .so (the same nats-client Cargo feature the dispatch relay
// stands in for). Without a drain, sealed events never leave the database:
// `event.kernel.<K>.*` stays silent and every consumer (web2, cklib ckOn
// binds) sees nothing. So this binary also drains the outbox.
//
// The drain runs as the dedicated least-privilege `ck_drainer` role
// (SELECT + DELETE on ckp.outbox only — created in the bundle's init.sql),
// NOT as ck_participant (the v3.9 floor keeps the participant role to
// ckp.dispatch alone) and NOT as a superuser. The loop polls every
// ~250ms, publishes each row to its stored subject in seq order with the
// row's headers, flushes to confirm server receipt, then deletes the
// confirmed rows. At-least-once: a crash between flush and delete
// re-publishes; consumers dedupe on the `Ck-Seq` header. There is no
// pg_notify on outbox insert, so polling is the mechanism (matches pgCK's
// documented bgworker tick loop).
//
// The drain shares the relay's retire posture: when pgCK ships the
// nats-client bgworker (multi-marker probe trips), the whole binary stands
// down and the native drain takes over. OCIGER_DISABLE_OUTBOX_DRAIN=1
// disables just the drain (for operators running their own).
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

	// Outbox drain tuning. Poll interval well under a second so the
	// seal→event path is sub-second end to end; batch bounds each tick.
	outboxDrainBatch    = 100
	outboxDrainInterval = 250 * time.Millisecond
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
	defer func() { _ = nc.Drain() }()

	sub, err := nc.Subscribe(relayInSubject, func(m *nats.Msg) {
		handleMsg(ctx, pool, nc, m)
	})
	if err != nil {
		log.Fatalf("subscribe %s failed: %v", relayInSubject, err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	log.Printf("dispatching %s → %s<K>.<verb> via ckp.dispatch", relayInSubject, relayOutPrefix)

	// Outbox drain — best-effort: a fresh v0.7.18+ volume has the ck_drainer
	// role; an older volume does not, in which case the drain stays off and
	// the dispatch path is unaffected (operators run their own drain there).
	if os.Getenv("OCIGER_DISABLE_OUTBOX_DRAIN") == "1" {
		log.Println("OCIGER_DISABLE_OUTBOX_DRAIN=1 — outbox drain off")
	} else {
		drainURL := getenv("OCIGER_PGCK_DRAIN_PG_URL",
			"postgres://ck_drainer@127.0.0.1:5432/postgres?sslmode=disable")
		if dpool := tryOpenPg(ctx, drainURL, "ociger-outbox-drain", 30*time.Second); dpool != nil {
			defer dpool.Close()
			go drainOutbox(ctx, dpool, nc)
		} else {
			log.Println("outbox drain disabled: could not connect as ck_drainer (pre-v0.7.18 volume?); dispatch unaffected")
		}
	}

	<-ctx.Done()
	log.Println("shutting down")
}

// drainOutbox polls ckp.outbox and republishes sealed events onto NATS,
// in seq order, until ctx is cancelled. See the package doc for the
// at-least-once contract.
func drainOutbox(ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn) {
	log.Printf("draining ckp.outbox → event.kernel.<K>.* every %s (as ck_drainer)", outboxDrainInterval)
	ticker := time.NewTicker(outboxDrainInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			drainBatch(ctx, pool, nc)
		}
	}
}

type outboxRow struct {
	seq     int64
	subject string
	payload []byte
	headers map[string]string
}

func drainBatch(ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn) {
	rows, err := pool.Query(ctx,
		"SELECT seq, subject, payload, headers FROM ckp.outbox ORDER BY seq ASC LIMIT $1",
		outboxDrainBatch)
	if err != nil {
		// ckp.outbox may not exist on the very first boot tick; stay quiet
		// and let the next tick find it.
		return
	}
	var batch []outboxRow
	for rows.Next() {
		var r outboxRow
		var headersRaw []byte
		if scanErr := rows.Scan(&r.seq, &r.subject, &r.payload, &headersRaw); scanErr != nil {
			rows.Close()
			return
		}
		if len(headersRaw) > 0 {
			_ = json.Unmarshal(headersRaw, &r.headers)
		}
		batch = append(batch, r)
	}
	rows.Close()
	if rows.Err() != nil || len(batch) == 0 {
		return
	}

	// Publish in seq order; stop at the first publish error so ordering is
	// preserved (the unpublished tail retries next tick). Then flush once to
	// confirm the server received the batch BEFORE deleting — at-least-once.
	var published []int64
	for _, r := range batch {
		hdr := nats.Header{}
		for k, v := range r.headers {
			hdr.Set(k, v)
		}
		if pubErr := nc.PublishMsg(&nats.Msg{Subject: r.subject, Data: r.payload, Header: hdr}); pubErr != nil {
			log.Printf("outbox publish seq=%d subject=%s failed: %v", r.seq, r.subject, pubErr)
			break
		}
		published = append(published, r.seq)
	}
	if len(published) == 0 {
		return
	}
	if flushErr := nc.FlushTimeout(2 * time.Second); flushErr != nil {
		// Not confirmed delivered — do NOT delete; next tick re-publishes
		// (consumers dedupe on Ck-Seq).
		log.Printf("outbox flush failed (%d events held): %v", len(published), flushErr)
		return
	}
	for _, seq := range published {
		if _, delErr := pool.Exec(ctx, "DELETE FROM ckp.outbox WHERE seq = $1", seq); delErr != nil {
			// Delivered but not deleted — re-publish next tick (at-least-once).
			log.Printf("outbox delete seq=%d failed: %v", seq, delErr)
			return
		}
	}
}

// handleMsg parses the inbound subject, calls ckp.dispatch, and
// publishes the typed reply.
//
// pgCK 0.3.x..0.4.x ships TWO `ckp.dispatch` overloads:
//
//	ckp.dispatch(verb text, payload jsonb)                                 — the GOVERNED 2-arg
//	ckp.dispatch(verb text, kernel_urn text, payload jsonb, identity text) — the CI-A-2 STUB
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

// tryOpenPg is the non-fatal sibling of mustOpenPg: it returns a connected
// pool, or nil if it cannot connect within deadline. Used for the outbox
// drain, which must not take the dispatch path down if the ck_drainer role
// is absent (an older volume).
func tryOpenPg(ctx context.Context, rawURL, appName string, within time.Duration) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(rawURL)
	if err != nil {
		log.Printf("drain pg url parse failed: %v", err)
		return nil
	}
	cfg.MaxConns = 2
	cfg.MinConns = 1
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = appName

	deadline := time.Now().Add(within)
	for {
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Printf("drain connected to %s", redactPassword(rawURL))
				return pool
			}
			pool.Close()
		}
		if time.Now().After(deadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
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
