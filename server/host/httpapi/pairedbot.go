package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/adams-shaun/gorge/server/host"
)

// pairedBotStatus is GET /api/tables/{t}/paired-bot: whether this table offers
// the paired bot. It reads configuration only, behind the same private-table
// fence as the other table reads.
func (h *handler) pairedBotStatus(w http.ResponseWriter, r *http.Request) {
	t := host.TableID(r.PathValue("t"))
	if !h.privateTableVisible(w, r, t) {
		return
	}
	st, err := h.reg.PairedBotStatus(t)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// advice is GET /api/tables/{t}/matches/{k}/advice?seq=: the paired bot's
// evaluation of the decision pending on the claim's seat. The seat comes from
// the claim alone; seq says which decision the client is showing, so an answer
// can never be for another. 409 covers a decision that has moved on, a seat
// with nothing pending and a table that does not offer the bot. A client that
// hangs up cancels the search.
func (h *handler) advice(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	claim, granted := h.claimForTable(w, r, t)
	if !granted {
		return
	}
	seq, err := strconv.ParseUint(r.URL.Query().Get("seq"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "seq must be the pending decision's sequence number")
		return
	}
	a, err := h.reg.Advice(r.Context(), t, k, claim.Seat, seq)
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		return // the client has gone; nobody reads a reply
	}
	if err != nil {
		writeSeatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
