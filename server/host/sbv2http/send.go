package sbv2http

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// maxSendBody bounds one answer, the same small bound server/host/manabrewhttp
// and server/host/httpapi use for their request bodies.
const maxSendBody = 64 << 10

// sendRequest is POST .../send's body: the open request's id and the candidate
// the human picked. The seat is never here; it is the token's claim.
type sendRequest struct {
	RequestID   string `json:"request_id"`
	CandidateID int    `json:"candidate_id"`
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
	if !h.hub.seat(seatKey{table: t, match: k, seat: claim.Seat}).answer(id, msg.CandidateID) {
		writeError(w, http.StatusConflict, "stale")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
