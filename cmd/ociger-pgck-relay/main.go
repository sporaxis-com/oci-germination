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
//      the literal string "input.kernel.pgCK.action" we assume the
//      upstream relay code is in the binary and stand down.
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
	probeMarker    = "input.kernel.pgCK.action"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("pgck-relay: ")

	if os.Getenv("OCIGER_DISABLE_PGCK_RELAY") == "1" {
		log.Println("OCIGER_DISABLE_PGCK_RELAY=1 — standing down (will not connect)")
		park()
	}

	if pgckHasRelayCode(pgckSoPath) {
		log.Printf("%s contains %q — assuming upstream nats-client build is live; standing down", pgckSoPath, probeMarker)
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

// pgckHasRelayCode reads the extension .so and looks for the wire
// constant only the upstream nats-client build embeds.
func pgckHasRelayCode(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(probeMarker))
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
