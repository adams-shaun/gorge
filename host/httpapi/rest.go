package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

func (h *handler) tables(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.reg.Tables())
}

func (h *handler) matches(w http.ResponseWriter, r *http.Request) {
	ms, err := h.reg.Matches(host.TableID(r.PathValue("t")))
	if err != nil {
		writeHostError(w, err)
		return
	}
	if ms == nil {
		ms = []protocol.MatchInfo{}
	}
	writeJSON(w, http.StatusOK, ms)
}

// matchKey parses {t} and {k}; a non-numeric k is a 400.
func matchKey(w http.ResponseWriter, r *http.Request) (host.TableID, int, bool) {
	k, err := strconv.Atoi(r.PathValue("k"))
	if err != nil || k < 1 {
		writeError(w, http.StatusBadRequest, "bad_request", "match index must be a positive integer")
		return "", 0, false
	}
	return host.TableID(r.PathValue("t")), k, true
}

// uintQuery parses an optional unsigned query parameter.
func uintQuery(w http.ResponseWriter, r *http.Request, name string) (uint64, bool, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, false, true
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", name+" must be a non-negative integer")
		return 0, false, false
	}
	return n, true, true
}

// claimSeat resolves the request through Options.Seat. nil means nobody may
// act as a seat — spectator-only, today's behaviour — so every request that
// names a seat on such a server is refused outright (403, not a nil-call
// panic); a non-nil resolver that declines the request is refused like an
// Authorize failure (401). The claim's seat is the only value the http layer
// trusts from the resolver: a request's ?seat= must equal it (seatFromQuery)
// and Pending/SubmitIntent act through it, so the resolver is the seat trust
// boundary exactly as Authorize is the request trust boundary.
func (h *handler) claimSeat(w http.ResponseWriter, r *http.Request) (SeatClaim, bool) {
	if h.opts.Seat == nil {
		writeError(w, http.StatusForbidden, "forbidden", "this server is spectator-only: no seat claims")
		return SeatClaim{}, false
	}
	claim, ok := h.opts.Seat(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "request does not hold a seat claim")
		return SeatClaim{}, false
	}
	return claim, true
}

// seatFromQuery resolves the optional ?seat= parameter. scoped is false (and
// ok true) when the parameter is absent: no claim is consulted and today's
// spectator behaviour runs byte-identical. A present ?seat= must name a seat
// the request actually holds — the resolver must answer, and the claim's seat
// must equal the requested one — so seat A can never read seat B's hand or
// decision through any endpoint that takes a seat parameter. ok false means
// the handler already wrote the rejection. (SubmitIntent takes no ?seat=; it
// acts through claimSeat alone, and the intent body's own Player field is the
// fence.)
func (h *handler) seatFromQuery(w http.ResponseWriter, r *http.Request) (seat state.PlayerID, scoped, ok bool) {
	raw := r.URL.Query().Get("seat")
	if raw == "" {
		return 0, false, true
	}
	n, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "seat must be a non-negative integer")
		return 0, false, false
	}
	claim, granted := h.claimSeat(w, r)
	if !granted {
		return 0, false, false
	}
	if claim.Seat != state.PlayerID(n) {
		writeError(w, http.StatusForbidden, "forbidden",
			fmt.Sprintf("claim holds seat %d, not the requested seat %d", claim.Seat, n))
		return 0, false, false
	}
	return claim.Seat, true, true
}

func (h *handler) view(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	seq, given, ok := uintQuery(w, r, "seq")
	if !ok {
		return
	}
	seat, scoped, ok := h.seatFromQuery(w, r)
	if !ok {
		return
	}
	if !given {
		ms, err := h.reg.Matches(t)
		if err != nil {
			writeHostError(w, err)
			return
		}
		found := false
		for _, m := range ms {
			if m.Match == k && m.Events > 0 {
				seq, found = uint64(m.Events-1), true
			}
		}
		if !found {
			writeHostError(w, host.ErrNotFound)
			return
		}
	}
	var (
		v   view.View
		err error
	)
	if scoped {
		v, err = h.reg.ViewAtSeat(t, k, seq, seat)
	} else {
		v, err = h.reg.ViewAt(t, k, seq)
	}
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *handler) events(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	since, _, ok := uintQuery(w, r, "since")
	if !ok {
		return
	}
	seat, scoped, ok := h.seatFromQuery(w, r)
	if !ok {
		return
	}
	var (
		evs []protocol.EventBody
		err error
	)
	if scoped {
		evs, err = h.reg.EventsSeat(t, k, since, seat)
	} else {
		evs, err = h.reg.Events(t, k, since)
	}
	if err != nil {
		writeHostError(w, err)
		return
	}
	if evs == nil {
		evs = []protocol.EventBody{}
	}
	writeJSON(w, http.StatusOK, evs)
}

// pending serves the decision currently asked of one seat (M2e-2). It needs
// ?seat= — there is no spectator notion of a pending decision — and the seat
// must be the claim's own, so a seat can never read another seat's pending
// decision. Rejections (match not live, seat not human, nothing pending) are
// 409 conflicts whose body carries the registry's own reason.
func (h *handler) pending(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	seat, scoped, ok := h.seatFromQuery(w, r)
	if !ok {
		return
	}
	if !scoped {
		writeError(w, http.StatusBadRequest, "bad_request", "pending requires a seat")
		return
	}
	d, err := h.reg.Pending(t, k, seat)
	if err != nil {
		writeSeatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// intent answers the pending decision through the claim's own seat. It takes
// no ?seat= parameter: the claim alone names the seat, and the intent body's
// own Player field is validated against the pending decision by
// decision.Decision.Validate inside the registry — the one fence — so a claim
// can never answer a decision asked of another seat. Every rejection becomes
// an HTTP status plus the registry's reason body (audit item 17); a 204 means
// the intent was accepted and the parked seat is free.
func (h *handler) intent(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	claim, granted := h.claimSeat(w, r)
	if !granted {
		return
	}
	var in decision.Intent
	if !decodeBody(w, r, &in) {
		return
	}
	if err := h.reg.SubmitIntent(t, k, claim.Seat, in); err != nil {
		writeSeatError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeBody reads a small JSON body; anything malformed is a 400.
func decodeBody(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "malformed JSON body: "+err.Error())
		return false
	}
	return true
}

// session resolves the body's session id; unknown is a 404 (it may have
// expired — the client must reconnect the stream to get a new one).
func (h *handler) session(w http.ResponseWriter, id string) (*host.Session, bool) {
	s, ok := h.reg.Session(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "unknown session "+id)
		return nil, false
	}
	return s, true
}

func (h *handler) subscribe(w http.ResponseWriter, r *http.Request) {
	var req protocol.Subscribe
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Mode != protocol.ModeOverview && req.Mode != protocol.ModeFocus {
		writeError(w, http.StatusBadRequest, "bad_request", "mode must be overview or focus")
		return
	}
	s, ok := h.session(w, req.Session)
	if !ok {
		return
	}
	if err := h.reg.Subscribe(s, host.TableID(req.Table), req.Mode); err != nil {
		if err == host.ErrNotFound {
			writeHostError(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req protocol.Unsubscribe
	if !decodeBody(w, r, &req) {
		return
	}
	s, ok := h.session(w, req.Session)
	if !ok {
		return
	}
	if err := h.reg.Unsubscribe(s, host.TableID(req.Table)); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
