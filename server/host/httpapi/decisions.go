package httpapi

import (
	"net/http"
)

// mountDecisions registers the review-only decision index route when
// Options.Decisions is set (match reviewer, design T4). A handler without it
// -- every live server -- registers nothing here.
func (h *handler) mountDecisions(mux *http.ServeMux) {
	if h.opts.Decisions == nil {
		return
	}
	mux.HandleFunc("GET /api/tables/{t}/matches/{k}/decisions", h.decisions)
	mux.HandleFunc("/api/tables/{t}/matches/{k}/decisions", methodNotAllowed)
}

// decisions serves the decision index: one protocol.ReviewDecision per
// intent of the recorded match, in log order.
func (h *handler) decisions(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	ds, err := h.opts.Decisions(t, k)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ds)
}
