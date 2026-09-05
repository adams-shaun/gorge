package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
)

func (h *handler) mountStream(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stream", h.stream)
}

// stream is one SSE connection. A Last-Event-ID of "<session>:<frame>"
// resumes that session from its ring; anything else — no header, an
// unknown session, or an id older than the ring — opens a fresh session
// and starts with hello (the client then re-subscribes).
func (h *handler) stream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}
	s, backlog := h.resume(r.Header.Get("Last-Event-ID"))
	fresh := s == nil
	if fresh {
		s = h.reg.OpenSession()
	}
	h.cancelGrace(s.ID) // a reconnect, resumed or fresh, cancels any pending close

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering in front of us, if any
	w.WriteHeader(http.StatusOK)

	write := func(f protocol.Frame) error { return writeFrame(w, s.ID, f) }

	if fresh {
		if err := write(h.reg.Hello(s)); err != nil {
			return
		}
	}
	for _, f := range backlog {
		if err := write(f); err != nil {
			return
		}
	}
	fl.Flush()

	widgets := time.NewTicker(h.opts.WidgetInterval)
	defer widgets.Stop()
	keep := time.NewTicker(h.opts.KeepAlive)
	defer keep.Stop()

	// terminal writes the overflow frame, when the close was in fact an
	// overflow (not a plain CloseSession), and flushes it.
	terminal := func() {
		if dropped, of := s.Overflowed(); of {
			ov, _ := protocol.NewFrame(protocol.TOverflow, "", 0, 0, protocol.Overflow{Dropped: dropped})
			if err := write(ov); err == nil {
				fl.Flush()
			}
		}
	}

	out := s.Out()
	for {
		select {
		case <-r.Context().Done():
			h.scheduleGrace(s.ID)
			return
		case f, open := <-out:
			if !open {
				terminal()
				return
			}
			if err := write(f); err != nil {
				h.scheduleGrace(s.ID)
				return
			}
			// Drain whatever else is already queued before flushing once,
			// so a burst of events costs one flush instead of many.
		drain:
			for {
				select {
				case f2, open2 := <-out:
					if !open2 {
						terminal()
						return
					}
					if err := write(f2); err != nil {
						h.scheduleGrace(s.ID)
						return
					}
				default:
					break drain
				}
			}
			fl.Flush()
		case <-widgets.C:
			ws := s.TakeWidgets()
			for _, f := range ws {
				if err := writeFrame(w, "", f); err != nil {
					h.scheduleGrace(s.ID)
					return
				}
			}
			if len(ws) > 0 {
				fl.Flush()
			}
		case <-keep.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				h.scheduleGrace(s.ID)
				return
			}
			fl.Flush()
		}
	}
}

// writeFrame emits one SSE message. The id line is "<session>:<frame>"
// (PL-6) and is omitted when the frame carries no id (widgets, the
// overflow frame) or session is "".
func writeFrame(w http.ResponseWriter, session string, f protocol.Frame) error {
	raw, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if f.ID != 0 && session != "" {
		if _, err := fmt.Fprintf(w, "id: %s:%d\n", session, f.ID); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", f.T, raw)
	return err
}

// resume parses Last-Event-ID. It returns the session and the frames to
// replay when the ring can serve them; nil, nil otherwise, telling the
// caller to start the client over with a fresh session.
func (h *handler) resume(header string) (*host.Session, []protocol.Frame) {
	sid, rest, ok := strings.Cut(header, ":")
	if !ok {
		return nil, nil
	}
	id, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		return nil, nil
	}
	s, ok := h.reg.Session(sid)
	if !ok {
		return nil, nil
	}
	frames, ok := s.Since(id)
	if !ok {
		h.reg.CloseSession(sid) // the client has lost sync; free the stale session
		return nil, nil
	}
	return s, frames
}

// scheduleGrace closes a disconnected session after ResumeGrace unless a
// resume arrives first (PL-6).
func (h *handler) scheduleGrace(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t, ok := h.grace[id]; ok {
		t.Stop()
	}
	h.grace[id] = time.AfterFunc(h.opts.ResumeGrace, func() {
		h.mu.Lock()
		delete(h.grace, id)
		h.mu.Unlock()
		h.reg.CloseSession(id)
	})
}

// cancelGrace stops a pending close for a session that just reconnected.
func (h *handler) cancelGrace(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t, ok := h.grace[id]; ok {
		t.Stop()
		delete(h.grace, id)
	}
}
