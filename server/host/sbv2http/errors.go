package sbv2http

import (
	"encoding/json"
	"net/http"
)

// errorBody is this transport's closed error shape: unsupported (the table is
// outside what v2 covers), stale (the answer names a request that is no longer
// open) and the ordinary HTTP-layer claim failures. It is deliberately tiny —
// the v2 protocol's own error envelope belongs to the agent line protocol, not
// to these opt-in routes.
type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorBody{Error: code})
}
