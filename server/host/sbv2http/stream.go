package sbv2http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// stream is GET .../stream: one SSE connection for the claim's own seat. On
// connect it sends the current request (or null); from then on every change of
// the seat's open request is pushed as a data line, including the null that
// marks a request answered, timed out or superseded. The connection carries no
// per-connection state, so a reconnect re-serves exactly what is still open.
func (h *handler) stream(w http.ResponseWriter, r *http.Request) {
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
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ss := h.hub.seat(seatKey{table: t, match: k, seat: claim.Seat})
	// last is the request id already written. A sentinel no real request can
	// have forces the initial frame (the null or the request already open), so
	// a client that connects after a decision parked is never left waiting for
	// a change it missed.
	last := ^uint64(0)
	for {
		req, wake := ss.snapshot()
		id := uint64(0)
		if req != nil {
			id = req.id
		}
		if id != last {
			last = id
			body := stateBody{}
			if req != nil {
				body.Request = &chooseRequest{
					RequestType: "choose",
					Protocol:    protocol.Name,
					RequestID:   fmt.Sprint(req.id),
					GameID:      req.ask.GameID,
					Decision:    req.ask.Decision,
				}
			}
			raw, err := json.Marshal(body)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
				return
			}
			fl.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-wake:
		}
	}
}
