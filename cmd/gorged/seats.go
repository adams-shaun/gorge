package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/host/httpapi"
	"github.com/adams-shaun/gorge/state"
)

// seatGate is the Options.Seat resolver of a -humans server. Each opaque
// token grants exactly one (table, seat) claim, never the same seat at every
// table. The token is process-local state: it never reaches an event, view,
// log or file. It is deliberately not general authentication.
type seatGate struct {
	mu           sync.RWMutex
	tokenToClaim map[string]httpapi.SeatClaim // bearer token -> the table and seat it holds
	claimTokens  map[httpapi.SeatClaim]string // claim -> token, for join-URL printing
}

// newSeatGate mints the human slots' tokens. With seed empty (the default)
// every slot gets its own crypto/rand 16-byte token, hex-encoded; with a
// non-empty seed — the -seat-token flag, documented for tests and local use
// — the first human slot takes the literal string and every later slot
// derives a deterministic variant "<tok>-<slot>", so a test can drive a
// seat without scraping stderr.
func newSeatGate(seed string, table host.TableID, humans []int) (*seatGate, error) {
	g := &seatGate{
		tokenToClaim: make(map[string]httpapi.SeatClaim, len(humans)),
		claimTokens:  make(map[httpapi.SeatClaim]string, len(humans)),
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
		claim := httpapi.SeatClaim{Table: table, Seat: state.PlayerID(s)}
		g.tokenToClaim[tok] = claim
		g.claimTokens[claim] = tok
	}
	return g, nil
}

// resolve implements httpapi.Options.Seat. The token is read from the
// Authorization: Bearer header first, then from the ?token= query — the
// browser needs the query form, and the Svelte client will use it. An
// absent or unknown token declines the request, which claimForTable turns
// into a 401; only a claim whose table and seat agree with the request
// reaches the table-scoped seat methods.
func (g *seatGate) resolve(r *http.Request) (httpapi.SeatClaim, bool) {
	tok := bearerToken(r)
	if tok == "" {
		tok = r.URL.Query().Get("token")
	}
	g.mu.RLock()
	claim, ok := g.tokenToClaim[tok]
	g.mu.RUnlock()
	return claim, ok
}

// token returns the claim's join token, for the startup print.
func (g *seatGate) token(table host.TableID, seat state.PlayerID) string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.claimTokens[httpapi.SeatClaim{Table: table, Seat: seat}]
}

// mint creates and records a fresh opaque token for exactly one table and
// seat. Old unbound tokens cannot be represented here and therefore fail the
// httpapi claimForTable fence after deployment.
func (g *seatGate) mint(table host.TableID, seat state.PlayerID) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("seats: minting token: %w", err)
	}
	tok := hex.EncodeToString(b[:])
	claim := httpapi.SeatClaim{Table: table, Seat: seat}
	g.mu.Lock()
	g.tokenToClaim[tok] = claim
	g.claimTokens[claim] = tok
	g.mu.Unlock()
	return tok, nil
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
