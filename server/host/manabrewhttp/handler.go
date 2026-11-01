package manabrewhttp

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/manabrew"
)

// SeatClaim names the (table, seat) an authenticated connection may act as.
// It is server/host/httpapi's own type, not a local twin: the
// claim boundary is one fence, shared by the native wire and by ManaBrew, so
// a token minted for one is meaningful to the other and the two can never
// disagree about which seat it names.
type SeatClaim = httpapi.SeatClaim

// Options configures the handler. Every duration defaults when zero (see
// withDefaults). Seat is the one authorization fence: nil is refused as
// forbidden on every request, the same "no seat claims here" shape
// server/host/httpapi uses for its spectator-only default, so a server that never
// wires ManaBrew support serves nothing an unauthenticated caller can act
// through. Whether it is an error to run at all with a nil Seat is a
// cmd/gorged concern, out of scope here.
type Options struct {
	// Seat resolves a request to the seat claim it may act for. nil refuses
	// every request with 403, matching server/host/httpapi's spectator-only shape.
	Seat func(*http.Request) (SeatClaim, bool)
	// Text is the card-text seam, forwarded to every internal/manabrew.Translator
	// this handler builds. nil (the default) means every CardDto carries empty text.
	Text manabrew.CardText
	// KeepAlive is the SSE comment cadence; 0 defaults to 15s, the same
	// default server/host/httpapi/sse.go uses.
	KeepAlive time.Duration
	// WriteTimeout bounds every single write to a stream connection (the
	// time allowlist's "write deadlines" paragraph, internal/archtest):
	// a client that stops reading must not block the goroutine serving it
	// forever. 0 defaults to 30s.
	WriteTimeout time.Duration
}

func (o Options) withDefaults() Options {
	if o.KeepAlive == 0 {
		o.KeepAlive = 15 * time.Second
	}
	if o.WriteTimeout == 0 {
		o.WriteTimeout = 30 * time.Second
	}
	return o
}

// handler is the unexported implementation behind NewHandler; tests in this
// package use newHandler to reach the seat-connection map directly.
type handler struct {
	reg  *host.Registry
	opts Options
	// submitIntentFn substitutes Registry.SubmitIntent in tests that need a
	// deterministic refusal the live engine cannot produce (a stream
	// auto-pass is always a legal pass). nil uses the registry.
	submitIntentFn func(host.TableID, int, state.PlayerID, decision.Intent) error

	mu    sync.Mutex
	conns map[connKey]*seatConn // lazily created, never removed (bounded by real (table,match,seat) tuples)
}

// newHandler builds the handler and its mux together, the same split
// server/host/httpapi uses so package tests can observe internals newHandler
// exposes that NewHandler's returned http.Handler does not.
func newHandler(r *host.Registry, o Options) (*handler, http.Handler) {
	h := &handler{reg: r, opts: o.withDefaults(), conns: map[connKey]*seatConn{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{t}/matches/{k}/stream", h.stream)
	mux.HandleFunc("POST /{t}/matches/{k}/send", h.send)
	mux.HandleFunc("GET /{t}/matches/{k}/state", h.state)
	return h, mux
}

// NewHandler builds the ManaBrew routes as their own http.Handler, rooted at
// whatever prefix the caller mounts it under (cmd/gorged mounts it at
// "/api/manabrew/v0/tables/" ahead of httpapi's own handler). It is the package's only exported
// constructor; callers that need the handler's internals (tests) use
// newHandler.
func NewHandler(r *host.Registry, o Options) http.Handler {
	_, mux := newHandler(r, o)
	return mux
}

// matchKey parses the {t}/{k} path values shared by every route; a
// non-numeric or non-positive k is a 400, matching server/host/httpapi's own
// matchKey.
func matchKey(w http.ResponseWriter, r *http.Request) (host.TableID, int, bool) {
	k, err := strconv.Atoi(r.PathValue("k"))
	if err != nil || k < 1 {
		writeError(w, http.StatusBadRequest, "bad_request", "match index must be a positive integer")
		return "", 0, false
	}
	return host.TableID(r.PathValue("t")), k, true
}

// claimForMatch is the one authorization fence every route here uses: nil
// Options.Seat means no ManaBrew seat claims exist on this server (403); an
// absent or unresolved claim is 401; and a claim for a different table is
// 403 — the exact claimForTable semantics server/host/httpapi's own seat-authorised
// routes use, applied to the ManaBrew path shape instead. These are plain
// HTTP-layer rejections, not mb.ProtocolError bodies: a claim failure means
// no ManaBrew message was ever accepted for processing, so none of the
// protocol's five closed error codes describes it.
func (h *handler) claimForMatch(w http.ResponseWriter, r *http.Request, table host.TableID) (SeatClaim, bool) {
	if h.opts.Seat == nil {
		writeError(w, http.StatusForbidden, "forbidden", "this server has no ManaBrew seat claims")
		return SeatClaim{}, false
	}
	claim, ok := h.opts.Seat(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "request does not hold a seat claim")
		return SeatClaim{}, false
	}
	if claim.Table != table {
		writeError(w, http.StatusForbidden, "forbidden", "claim does not hold a seat at this table")
		return SeatClaim{}, false
	}
	return claim, true
}
