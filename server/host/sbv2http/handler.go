package sbv2http

import (
	"net/http"
	"strconv"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// SeatClaim names the (table, seat) an authenticated request may act as. It is
// httpapi's own type, not a local twin: the claim boundary is one fence shared
// with the native wire and ManaBrew, so a token minted for one is meaningful to
// the others and they can never disagree about which seat it names.
type SeatClaim = httpapi.SeatClaim

// Options configures the handler. Seat is the only authorization fence: nil
// refuses every request with 403, the same spectator-only default
// server/host/httpapi uses.
type Options struct {
	// Seat resolves a request to the seat claim it may act for. nil refuses
	// every request with 403.
	Seat func(*http.Request) (SeatClaim, bool)
}

// handler is the unexported implementation behind NewHandler; tests in this
// package use newHandler to reach the hub and registry directly.
type handler struct {
	reg  *host.Registry
	hub  *Hub
	opts Options
}

func newHandler(r *host.Registry, hub *Hub, o Options) (*handler, http.Handler) {
	h := &handler{reg: r, hub: hub, opts: o}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{t}/matches/{k}/stream", h.stream)
	mux.HandleFunc("POST /{t}/matches/{k}/send", h.send)
	mux.HandleFunc("GET /{t}/matches/{k}/state", h.state)
	return h, mux
}

// NewHandler builds the SpellBench v2 routes as their own http.Handler, rooted
// at whatever prefix the caller mounts it under. server/gorged mounts it at
// "/api/sbv2/v1/tables/" (sbv2RoutePrefix) only while -sbv2 is on.
func NewHandler(r *host.Registry, hub *Hub, o Options) http.Handler {
	_, mux := newHandler(r, hub, o)
	return mux
}

// matchKey parses the {t}/{k} path values shared by every route; a
// non-numeric or non-positive k is a 400.
func matchKey(w http.ResponseWriter, r *http.Request) (host.TableID, int, bool) {
	k, err := strconv.Atoi(r.PathValue("k"))
	if err != nil || k < 1 {
		writeError(w, http.StatusBadRequest, "bad_request")
		return "", 0, false
	}
	return host.TableID(r.PathValue("t")), k, true
}

// claimForMatch is the one authorization fence every route here uses: nil
// Options.Seat means no v2 seat claims exist on this server (403); an absent or
// unresolved claim is 401; and a claim for a different table is 403. The seat
// is the claim's, never the request body's or a path component's.
func (h *handler) claimForMatch(w http.ResponseWriter, r *http.Request, table host.TableID) (SeatClaim, bool) {
	if h.opts.Seat == nil {
		writeError(w, http.StatusForbidden, "forbidden")
		return SeatClaim{}, false
	}
	claim, ok := h.opts.Seat(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return SeatClaim{}, false
	}
	if claim.Table != table {
		writeError(w, http.StatusForbidden, "forbidden")
		return SeatClaim{}, false
	}
	return claim, true
}

// covers reports whether the table is inside what v2 covers: exactly two seats,
// constructed (not Commander), and automatic mana. A table it does not cover
// answers 409 unsupported; an unknown table answers 404. The table's public
// configuration (protocol.TableInfo) is the only input, so no match needs to
// have started for the probe to answer.
func (h *handler) covers(w http.ResponseWriter, table host.TableID) bool {
	for _, t := range h.reg.Tables() {
		if t.ID != string(table) {
			continue
		}
		if t.Seats != 2 || t.Format != "constructed" || !t.AutoMana {
			writeError(w, http.StatusConflict, "unsupported")
			return false
		}
		return true
	}
	writeError(w, http.StatusNotFound, "not_found")
	return false
}

// stateBody is the one JSON document the state route returns and the stream
// route pushes: the open choose request for the caller's seat, or null.
type stateBody struct {
	Request *chooseRequest `json:"request"`
}

// chooseRequest mirrors the SpellBench v2 choose request: the request id the
// client echoes in POST .../send, the game id, and the typed seat decision.
type chooseRequest struct {
	RequestType string                `json:"request_type"`
	Protocol    string                `json:"protocol"`
	RequestID   string                `json:"request_id"`
	GameID      string                `json:"game_id"`
	Decision    protocol.SeatDecision `json:"decision"`
}

func (h *handler) stateBody(key seatKey) stateBody {
	req, _ := h.hub.seat(key).snapshot()
	if req == nil {
		return stateBody{}
	}
	return stateBody{Request: &chooseRequest{
		RequestType: "choose",
		Protocol:    protocol.Name,
		RequestID:   strconv.FormatUint(req.id, 10),
		GameID:      req.ask.GameID,
		Decision:    req.ask.Decision,
	}}
}
