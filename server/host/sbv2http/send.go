package sbv2http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
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
// seat's existing native paths; choose_name answers a card-name ask with any
// name the engine offered, including those past the 4096-candidate window of
// the seat_decision. At most one may be set.
type humanAction struct {
	Concede    bool   `json:"concede"`
	Undo       bool   `json:"undo"`
	ChooseName string `json:"choose_name,omitempty"`
}

// actions counts the actions set on a.
func (a *humanAction) actions() int {
	n := 0
	for _, set := range []bool{a.Concede, a.Undo, a.ChooseName != ""} {
		if set {
			n++
		}
	}
	return n
}

// nameIntent is the one home for choose_name's legal-answer rule: name must be
// the exact Label of a Kind "name" option of the pending native decision d,
// and the one-choice intent for it must pass d.Validate.
func nameIntent(d *decision.Decision, name string) (decision.Intent, error) {
	if d == nil || d.Kind != decision.KChoose {
		return decision.Intent{}, errors.New("no pending card-name decision")
	}
	for _, o := range d.Options {
		if o.Kind != "name" || o.Label != name {
			continue
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}
		if err := d.Validate(in); err != nil {
			return decision.Intent{}, err
		}
		return in, nil
	}
	return decision.Intent{}, errors.New("the pending decision does not offer that name")
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
		if msg.Human.actions() > 1 {
			writeError(w, http.StatusBadRequest, "bad_request")
			return
		}
		if !h.hub.seat(seatKey{table: t, match: k, seat: claim.Seat}).open(id) {
			writeError(w, http.StatusConflict, "stale")
			return
		}
	}
	if msg.Human != nil && msg.Human.ChooseName != "" {
		h.chooseName(w, claim.Seat, t, k, id, msg.Human.ChooseName)
		return
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

// chooseName answers the open card-name request with name. The name is
// resolved against the PENDING native decision, not the pose's candidates, so
// every offered name is answerable. A name ask folded into a priority pose's
// follow-ups is a different native decision than the pose's: its seq does not
// match, so it is stale here and the client falls back to candidate_id or the
// native routes.
func (h *handler) chooseName(w http.ResponseWriter, seat state.PlayerID, t host.TableID, k int, id uint64, name string) {
	ss := h.hub.seat(seatKey{table: t, match: k, seat: seat})
	req, ok := ss.openRequest(id)
	if !ok {
		writeError(w, http.StatusConflict, "stale")
		return
	}
	d, err := h.reg.Pending(t, k, seat)
	if err != nil || d == nil || d.Seq != req.ask.Pose.Native.Seq {
		writeError(w, http.StatusConflict, "stale")
		return
	}
	in, err := nameIntent(d, name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := h.reg.SubmitIntent(t, k, seat, in); err != nil {
		writeError(w, http.StatusConflict, "stale")
		return
	}
	ss.markAnswered(id)
	w.WriteHeader(http.StatusNoContent)
}
