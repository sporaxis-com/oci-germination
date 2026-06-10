// ociger-pgck-relay — bus-level shim that performs the
// input.kernel.pgCK.action.>  →  event.kernel.pgCK.<verb>  fan-out
// while pgCK's published pgck.so is built without the `nats-client`
// Cargo feature.
//
// Contract is identical to pgCK's own src/nats_client.rs:
//
//   RELAY_IN_SUBJECT  = "input.kernel.pgCK.action.>"
//   RELAY_IN_PREFIX   = "input.kernel.pgCK.action."
//   RELAY_OUT_PREFIX  = "event.kernel.pgCK."
//
// Each MSG received on RELAY_IN_SUBJECT is re-PUBlished verbatim
// (payload + headers) to RELAY_OUT_PREFIX + <verb>.
//
// Self-disable conditions (so the shim coexists cleanly with an
// eventual upstream nats-client build):
//
//   1. env OCIGER_DISABLE_PGCK_RELAY=1 → block forever, never connect.
//   2. boot probe: if /usr/lib/postgresql/17/lib/pgck.so contains
//      BOTH "RELAY_OUT_PREFIX" AND "async_nats::" — the discriminating
//      conjunction of (a) the relay-code constant from
//      src/nats_client.rs and (b) any reachable symbol from the
//      async_nats Rust crate — we assume the upstream `nats-client`
//      Cargo feature is compiled in and stand down.
//
// The single-string probe used in v0.7.5–v0.7.7 was a false-positive
// against pgCK v0.3.x (the registry seed and SQL comments contain the
// literal `input.kernel.pgCK.action` without any relay code present).
// The two-marker conjunction discriminates "feature compiled in" from
// "feature off" because either marker alone could appear in unrelated
// text; ratified with pgCK on 2026-06-10 (the markers will be present
// in any natural Rust release build with the feature enabled).
//
// Standing down means "park as a no-op longrun" rather than exit so
// s6 doesn't restart-loop us.
package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	relayInSubject = "input.kernel.pgCK.action.>"
	relayInPrefix  = "input.kernel.pgCK.action."
	relayOutPrefix = "event.kernel.pgCK."
	pgckSoPath     = "/usr/lib/postgresql/17/lib/pgck.so"
	// The two markers conjoined make the retire-condition discriminating:
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

	url := getenv("OCIGER_PGCK_RELAY_URL", "nats://127.0.0.1:4222")
	log.Printf("connecting to %s", url)

	nc, err := nats.Connect(url,
		nats.Name("ociger-pgck-relay"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, e error) {
			if e != nil {
				log.Printf("disconnected: %v", e)
			}
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			log.Printf("reconnected to %s", c.ConnectedUrl())
		}),
	)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer nc.Drain()

	sub, err := nc.Subscribe(relayInSubject, func(m *nats.Msg) {
		verb := strings.TrimPrefix(m.Subject, relayInPrefix)
		if verb == m.Subject || verb == "" {
			return
		}
		out := relayOutPrefix + verb
		fwd := &nats.Msg{Subject: out, Data: m.Data, Header: m.Header}
		if err := nc.PublishMsg(fwd); err != nil {
			log.Printf("publish %s failed: %v", out, err)
		}
	})
	if err != nil {
		log.Fatalf("subscribe %s failed: %v", relayInSubject, err)
	}
	defer sub.Unsubscribe()

	log.Printf("relaying %s → %s<verb>", relayInSubject, relayOutPrefix)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	<-ctx.Done()
	log.Println("shutting down")
}

// pgckHasRelayCode reads the extension .so and returns true only when
// BOTH discriminating markers are present: the relay-code constant
// (RELAY_OUT_PREFIX, from src/nats_client.rs — always lives in .rodata
// because it's a `const &str`) AND a symbol from the async-nats crate
// path (async_nats:: — appears in the binary symbol table from any
// reachable call). Either alone can appear incidentally in SQL
// comments or unrelated source text; the conjunction discriminates
// "feature compiled in" from "feature off". Verified against pgCK
// v0.3.3 (both markers absent → shim runs, correct).
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
