package sbv2http

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/adams-shaun/gorge/server/manabrew"
)

// maxSendBody bounds one answer, the same small bound server/host/manabrewhttp
// and server/host/httpapi use for their request bodies.
const maxSendBody = 64 << 10

// sendRequest is POST .../send's body: the open request's id, the candidate
// the human picked, or one of the x_gorge_human_v1 actions. The seat is never
// here; it is the token's claim.
type sendRequest struct {
	RequestID   string       `json:"request_id"`
	CandidateID *int         `json:"candidate_id"`
	Human       *humanAction `json:"x_gorge_human_v1"`
}

// humanAction is the write half of x_gorge_human_v1. Concede and undo take the
// seat's existing native paths; at most one may be set.
type humanAction struct {
	Concede bool `json:"concede"`
	Undo    bool `json:"undo"`
}

// send is POST .../send: it answers the claim seat's open request through the
// V2HumanSeat tap. The first answer wins; an answer naming a request that is
// no longer open — already answered, superseded, timed out, or another seat's
// — is 409 stale and changes nothing.
func (h *handler) send(w http.ResponseWriter, r *http.Request) {
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

	body, err := io.ReadAll(io.LimitReader(r.Body, maxSendBody+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if len(body) > maxSendBody {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	var msg sendRequest
	if err := json.Unmarshal(body, &msg); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	id, err := strconv.ParseUint(msg.RequestID, 10, 64)
	if err != nil {
		writeError(w, http.StatusConflict, "stale")
		return
	}
	if msg.Human != nil {
		if msg.Human.Undo && msg.Human.Concede {
			writeError(w, http.StatusBadRequest, "bad_request")
			return
		}
		if !h.hub.seat(seatKey{table: t, match: k, seat: claim.Seat}).open(id) {
			writeError(w, http.StatusConflict, "stale")
			return
		}
	}
	if msg.Human != nil && msg.Human.Undo {
		if err := h.reg.Undo(t, k, claim.Seat); err != nil {
			writeError(w, http.StatusConflict, "stale")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if msg.Human != nil && msg.Human.Concede {
		d, err := h.reg.Pending(t, k, claim.Seat)
		if err != nil {
			writeError(w, http.StatusConflict, "stale")
			return
		}
		in, err := manabrew.ConcedeIntent(d)
		if err != nil {
			writeError(w, http.StatusConflict, "stale")
			return
		}
		if err := h.reg.SubmitIntent(t, k, claim.Seat, in); err != nil {
			writeError(w, http.StatusConflict, "stale")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if msg.CandidateID == nil || !h.hub.seat(seatKey{table: t, match: k, seat: claim.Seat}).answer(id, *msg.CandidateID) {
		writeError(w, http.StatusConflict, "stale")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
