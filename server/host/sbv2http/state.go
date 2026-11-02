package sbv2http

import (
	"net/http"
)

// leavesAsks reads a v2 read's asks=native: this client answers, on the native
// routes, the asks the codec otherwise answers itself.
func leavesAsks(r *http.Request) bool {
	return r.URL.Query().Get("asks") == "native"
}

// state is GET .../state: the current v2 choose request for the caller's seat,
// or {"request":null} when the seat is between decisions (or the codec left the
// decision to the native wire). It reads exactly what the stream's initial
// frame sends and answers 409 unsupported for a table v2 does not cover. It
// attaches the seat, so the first read can come before its first request.
func (h *handler) state(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	claim, ok := h.claimForMatch(w, r, t)
	if !ok {
		return
	}
	if !h.covers(w, t) {
		return
	}
	key := seatKey{table: t, match: k, seat: claim.Seat}
	h.hub.seat(key).attach(leavesAsks(r))
	writeJSON(w, http.StatusOK, h.stateBody(key))
}
