package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/adams-shaun/gorge/host/httpapi"
	"github.com/adams-shaun/gorge/state"
)

// seatGate is the Options.Seat resolver of a -humans server: it mints one
// opaque token per human slot at startup and grants exactly the seat whose
// token a request carries. A token minted for seat 0 satisfies ?seat=0 on
// every table — SeatClaim carries no table (R-E3-1) — which is precisely
// why only table t1 may have humans, and why the resolver is the real
// enforcement behind M2e-2's claim-vs-requested comparison rather than the
// rubber stamp that would make that comparison tautological.
//
// The token is startup state, not game state: it never reaches an event, a
// view, the log or a file (determinism). This is deliberately not
// authentication — gorged binds a plain :8080 with no accounts, TLS,
// sessions, cookies, expiry or rate limiting, and the gate must not grow
// into any of those.
type seatGate struct {
	tokenToSeat map[string]state.PlayerID // bearer token -> the seat it holds
	seatTokens  map[state.PlayerID]string // seat -> its token, for the join-URL print
}

// newSeatGate mints the human slots' tokens. With seed empty (the default)
// every slot gets its own crypto/rand 16-byte token, hex-encoded; with a
// non-empty seed — the -seat-token flag, documented for tests and local use
// — the first human slot takes the literal string and every later slot
// derives a deterministic variant "<tok>-<slot>", so a test can drive a
// seat without scraping stderr.
func newSeatGate(seed string, humans []int) (*seatGate, error) {
	g := &seatGate{
		tokenToSeat: make(map[string]state.PlayerID, len(humans)),
		seatTokens:  make(map[state.PlayerID]string, len(humans)),
	}
	for i, s := range humans {
		tok := seed
		if seed == "" {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				return nil, fmt.Errorf("seats: minting token: %w", err)
			}
			tok = hex.EncodeToString(b[:])
		} else if i > 0 {
			tok = fmt.Sprintf("%s-%d", seed, s)
		}
		g.tokenToSeat[tok] = state.PlayerID(s)
		g.seatTokens[state.PlayerID(s)] = tok
	}
	return g, nil
}

// resolve implements httpapi.Options.Seat. The token is read from the
// Authorization: Bearer header first, then from the ?token= query — the
// browser needs the query form, and the Svelte client will use it. An
// absent or unknown token declines the request, which claimSeat turns into
// a 401; only a claim whose seat agrees with the request's own ?seat=
// reaches the seat-scoped methods (seatFromQuery).
func (g *seatGate) resolve(r *http.Request) (httpapi.SeatClaim, bool) {
	tok := bearerToken(r)
	if tok == "" {
		tok = r.URL.Query().Get("token")
	}
	seat, ok := g.tokenToSeat[tok]
	if !ok {
		return httpapi.SeatClaim{}, false
	}
	return httpapi.SeatClaim{Seat: seat}, true
}

// token returns the slot's join token, for the startup print.
func (g *seatGate) token(seat state.PlayerID) string {
	return g.seatTokens[seat]
}

// joinHost renders a listener address as something a browser can open. A
// listener bound to the default ":8080" reports its address as "[::]:8080" —
// the unspecified address, which is a legal thing to listen on and not a
// legal thing to dial — so a join URL built straight from ln.Addr() cannot be
// pasted anywhere, which is the only reason it is printed. Substitute the
// loopback host in that case and leave an explicit -addr host alone.
func joinHost(addr net.Addr) string {
	s := addr.String()
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return s
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return s
}

// bearerToken reads an Authorization: Bearer <tok> header; absent returns "".
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}
