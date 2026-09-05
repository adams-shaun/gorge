package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
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

func (h *handler) view(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	seq, given, ok := uintQuery(w, r, "seq")
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
	v, err := h.reg.ViewAt(t, k, seq)
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
	evs, err := h.reg.Events(t, k, since)
	if err != nil {
		writeHostError(w, err)
		return
	}
	if evs == nil {
		evs = []protocol.EventBody{}
	}
	writeJSON(w, http.StatusOK, evs)
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
