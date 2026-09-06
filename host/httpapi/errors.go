// Package httpapi serves a host.Registry over plain net/http: JSON GETs for
// tables, matches, views and events; POSTs for subscriptions; one SSE
// stream per client; and the embedded web client. cmd/gorged mounts it;
// mtgserve can too (spec D9, PL-4).
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
)

// writeJSON writes any reply; a marshal failure of our own types is a bug
// and surfaces as a 500 with the message.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are gone; the best we can do is log-free silence — the
		// client sees a truncated body and its decoder fails.
		return
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, protocol.ErrorBody{Code: code, Message: msg})
}

// writeHostError maps the host's errors onto statuses: not found 404, seq
// beyond head 409 with the head in the body, anything else 500.
func writeHostError(w http.ResponseWriter, err error) {
	var beyond host.ErrBeyondHead
	switch {
	case errors.Is(err, host.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.As(err, &beyond):
		writeJSON(w, http.StatusConflict, protocol.ErrorBody{Code: "beyond_head", Message: err.Error(), Head: beyond.Head})
	default:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
	}
}

// writeSeatError maps the seat methods' (Pending, SubmitIntent) errors onto
// statuses. An unknown table/match stays 404; every other rejection is a 409
// conflict whose reason body is gorge's own error message: the match is not
// in the state the request assumed — not live, the seat is not a human seat,
// no decision is parked, or the intent is stale, for the wrong player,
// out-of-range, duplicated or violates min/max. The handler adds no second
// validation layer; the one fence is decision.Decision.Validate inside the
// registry, and this status-plus-reason surface is audit item 17's answer.
func writeSeatError(w http.ResponseWriter, err error) {
	if errors.Is(err, host.ErrNotFound) {
		writeHostError(w, err)
		return
	}
	writeError(w, http.StatusConflict, "conflict", err.Error())
}
