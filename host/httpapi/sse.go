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

	// Every return from here on means the client is gone; arm the grace
	// timer so the session is reclaimed unless a resume arrives first. One
	// deferred path covers the initial hello/backlog flush as well as every
	// write failure in the main loop below.
	disconnected := false
	defer func() {
		if disconnected {
			h.scheduleGrace(s.ID)
		}
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering in front of us, if any
	w.WriteHeader(http.StatusOK)

	write := func(f protocol.Frame) error { return writeFrame(w, s.ID, f) }

	if fresh {
		if err := write(h.reg.Hello(s)); err != nil {
			disconnected = true
			return
		}
	}
	for _, f := range backlog {
		if err := write(f); err != nil {
			disconnected = true
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
			disconnected = true
			return
		case f, open := <-out:
			if !open {
				terminal()
				return
			}
			if err := write(f); err != nil {
				disconnected = true
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
						disconnected = true
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
					disconnected = true
					return
				}
			}
			if len(ws) > 0 {
				fl.Flush()
			}
		case <-keep.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				disconnected = true
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

// graceTimer is one pending disconnect-close for a session. gen lets a
// stale timer's callback recognise that it has been superseded by a newer
// scheduleGrace or cancelled by a reconnect.
type graceTimer struct {
	gen   uint64
	timer *time.Timer
}

// scheduleGrace closes a disconnected session after ResumeGrace unless a
// resume arrives first (PL-6). The callback re-checks under h.mu that its
// generation is still the one registered for the session: time.AfterFunc's
// contract is that once Stop returns false the callback has already started
// and cannot be aborted, so a reconnect that won the lock first must not
// have its session closed by the stale callback.
func (h *handler) scheduleGrace(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if g, ok := h.grace[id]; ok {
		g.timer.Stop()
	}
	h.graceGen++
	g := &graceTimer{gen: h.graceGen}
	g.timer = time.AfterFunc(h.opts.ResumeGrace, func() { h.graceExpired(id, g.gen) })
	h.grace[id] = g
}

// graceExpired runs on the timer goroutine. It closes the session only if
// this timer's generation is still the one registered for id — i.e. no
// reconnect cancelled it and no newer scheduleGrace replaced it.
func (h *handler) graceExpired(id string, gen uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.grace[id]
	if !ok || g.gen != gen {
		return
	}
	delete(h.grace, id)
	h.reg.CloseSession(id)
}

// cancelGrace stops a pending close for a session that just reconnected.
func (h *handler) cancelGrace(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if g, ok := h.grace[id]; ok {
		g.timer.Stop()
		delete(h.grace, id)
	}
}
