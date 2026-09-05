package host

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// FrameType is protocol's own discriminator, used throughout this file.
type FrameType = protocol.FrameType

// frame builds an addressed frame; a marshal failure of our own structs is
// a bug, not a runtime condition.
func frame(t FrameType, tab *table, k int, seq uint64, body any) protocol.Frame {
	f, err := protocol.NewFrame(t, string(tab.cfg.ID), k, seq, body)
	if err != nil {
		panic("host: frame: " + err.Error())
	}
	return f
}

// head is the seq of the last event, or 0 for an empty log.
func head(m *match) uint64 {
	if n := len(m.e.L.Events); n > 0 {
		return uint64(n - 1)
	}
	return 0
}

// snapshotFrame is the whole board at head in the table's visibility plus
// the turn starts. Called with m.mu held for reading.
func (r *Registry) snapshotFrame(t *table, m *match) protocol.Frame {
	v := view.ProjectFor(m.e.G, m.e, view.NoSeat, t.cfg.Spectator, nil)
	return frame(protocol.TSnapshot, t, m.k, head(m), protocol.Snapshot{
		View: v, TurnStarts: append([]uint64(nil), m.turnStarts...), Head: head(m)})
}

// widgetFrame is the overview cell. Called with m.mu held for reading.
func (r *Registry) widgetFrame(t *table, m *match, last string) protocol.Frame {
	g := m.e.G
	w := protocol.Widget{Turn: g.Turn, Step: g.Step.String(), Phase: view.PhaseOf(g.Step),
		Active: uint8(g.Active), Priority: uint8(g.Priority), StackDepth: len(g.Stack), Last: last, State: m.state}
	for _, p := range g.Players {
		w.Life = append(w.Life, p.Life)
		w.Lost = append(w.Lost, p.Lost)
	}
	if w.Life == nil {
		w.Life, w.Lost = []int32{}, []bool{}
	}
	return frame(protocol.TWidget, t, m.k, head(m), w)
}

// eventBodiesFor redacts and describes evs against g for one viewer, the
// state that produced them (RedactEventsFor's convention): the viewer/vis
// pair is exactly what RedactEventsFor takes, so Events (spectator) and
// EventsSeat (seat) and the fan-out (the table's spectator) all share this
// one body and can only drift through that pair. Describe runs on the
// REDACTED event so a hidden card's name never reaches the line.
//
// Takes g and evs rather than a live (*table, *match) (fix round 1, FL-42)
// so a caller that cannot hold m.mu for the whole call — Events, over a
// caller-controlled since that can span the entire log — can copy g
// (Clone) and evs (a slice copy) under a brief read lock and format them
// afterwards. fanout/onMatchStart/onMatchEnd still call this under their
// own already-held lock, passing m.e.G and m.e.L.Events[from:] directly:
// no clone needed there, since they never release the lock mid-call.
func eventBodiesFor(viewer state.PlayerID, vis view.Visibility, g *state.Game, evs []events.Event) []protocol.EventBody {
	red := view.RedactEventsFor(g, evs, viewer, vis)
	out := make([]protocol.EventBody, 0, len(red))
	for _, ev := range red {
		out = append(out, protocol.EventBody{Event: protocol.EventFrom(ev), Line: view.Describe(g, ev)})
	}
	return out
}

// lastLine is the transcript line of the most recent described event that
// has one, for widget.last; prev carries the previous value forward when
// nothing in bodies has a line.
func lastLine(bodies []protocol.EventBody, prev string) string {
	for i := len(bodies) - 1; i >= 0; i-- {
		if bodies[i].Line != "" {
			return bodies[i].Line
		}
	}
	return prev
}

// tailFrom bounds a one-time end-of-match rescan to (at most) the last n
// events, so finding the closing transcript line costs a small constant
// regardless of how long the match ran, rather than redacting/describing
// the whole log again.
func tailFrom(m *match, n int) int {
	if total := len(m.e.L.Events); total > n {
		return total - n
	}
	return 0
}

// hasMode reports whether modes contains mode.
func hasMode(modes []string, mode string) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

// fanout delivers one burst to every subscribed session: focus sessions
// get the events (and the pending decision, if any) as frames; overview
// sessions get a coalesced widget. It never blocks on a client.
//
// Building the per-event frames (evFrames) or the widget is gated on
// whether a subscriber of that mode actually exists, since a burst runs up
// to tens of thousands of times per match; eventBodies itself still runs
// whenever anyone is subscribed, focus or overview, since both need it
// (evFrames from it directly, the widget for its transcript line).
func (r *Registry) fanout(t *table, m *match, before int) {
	ss, modes := r.sessionsFor(t.cfg.ID)
	if len(ss) == 0 {
		return
	}
	focus, overview := hasMode(modes, protocol.ModeFocus), hasMode(modes, protocol.ModeOverview)

	m.mu.RLock()
	var evFrames []protocol.Frame
	var widget protocol.Frame
	bodies := eventBodiesFor(view.NoSeat, t.cfg.Spectator, m.e.G, m.e.L.Events[before:])
	if focus {
		evFrames = make([]protocol.Frame, 0, len(bodies))
		for _, b := range bodies {
			evFrames = append(evFrames, frame(protocol.TEvent, t, m.k, b.Event.Seq, b))
		}
	}
	t.lastLine = lastLine(bodies, t.lastLine)
	if overview {
		widget = r.widgetFrame(t, m, t.lastLine)
	}
	var decision *protocol.Frame
	if focus {
		if d := m.e.Pending(); d != nil {
			f := frame(protocol.TDecision, t, m.k, head(m), protocol.DecisionBody{Player: uint8(d.Player), Kind: string(d.Kind), Prompt: d.Prompt})
			decision = &f
		}
	}
	m.mu.RUnlock()

	// fanMu serialises this push loop against a focus Subscribe's own
	// snapshot build+push (Ruling FL-30) — see host/session.go's Subscribe
	// and host/table.go's fanMu doc. push never blocks, so this never
	// parks the match loop on a client.
	t.fanMu.Lock()
	for i, s := range ss {
		switch modes[i] {
		case protocol.ModeFocus:
			for _, f := range evFrames {
				if !s.push(f) {
					break
				}
			}
			if decision != nil {
				s.push(*decision)
			}
		case protocol.ModeOverview:
			s.setWidget(t.cfg.ID, widget)
		}
	}
	t.fanMu.Unlock()
	r.dropOverflowed(ss)
}

// onMatchStart announces a match to every subscriber and gives focus
// subscribers their first snapshot. Genesis events (before the first
// intent) are already inside that snapshot's head, so a focus client
// starts from the snapshot rather than replaying them as frames.
func (r *Registry) onMatchStart(t *table, m *match) {
	ss, modes := r.sessionsFor(t.cfg.ID)
	if len(ss) == 0 {
		return
	}
	focus, overview := hasMode(modes, protocol.ModeFocus), hasMode(modes, protocol.ModeOverview)

	m.mu.RLock()
	start := frame(protocol.TMatchStart, t, m.k, 0, protocol.MatchStart{Seats: m.seats, Seed: m.seed, Spectator: t.cfg.Spectator.String()})
	var snap, widget protocol.Frame
	if focus {
		snap = r.snapshotFrame(t, m)
	}
	if overview {
		bodies := eventBodiesFor(view.NoSeat, t.cfg.Spectator, m.e.G, m.e.L.Events)
		t.lastLine = lastLine(bodies, t.lastLine)
		widget = r.widgetFrame(t, m, t.lastLine)
	}
	m.mu.RUnlock()

	t.fanMu.Lock()
	for i, s := range ss {
		s.push(start)
		if modes[i] == protocol.ModeFocus {
			s.push(snap)
		} else {
			s.setWidget(t.cfg.ID, widget)
		}
	}
	t.fanMu.Unlock()
	r.dropOverflowed(ss)
}

// onMatchEnd sends match_end (any final state) to every subscriber and a
// final widget to overview ones. The match is archived to disk first
// (Task 12), so the end frame and the sidecar agree.
func (r *Registry) onMatchEnd(t *table, m *match) {
	r.archive(t, m)
	ss, modes := r.sessionsFor(t.cfg.ID)
	if len(ss) == 0 {
		return
	}
	overview := hasMode(modes, protocol.ModeOverview)

	m.mu.RLock()
	end := frame(protocol.TMatchEnd, t, m.k, head(m), protocol.MatchEnd{Result: m.result, Winner: m.winner, Head: m.head})
	var widget protocol.Frame
	if overview {
		bodies := eventBodiesFor(view.NoSeat, t.cfg.Spectator, m.e.G, m.e.L.Events[tailFrom(m, 64):])
		t.lastLine = lastLine(bodies, t.lastLine)
		widget = r.widgetFrame(t, m, t.lastLine)
	}
	m.mu.RUnlock()

	t.fanMu.Lock()
	for i, s := range ss {
		s.push(end)
		if modes[i] == protocol.ModeOverview {
			s.setWidget(t.cfg.ID, widget)
		}
	}
	t.fanMu.Unlock()
	r.dropOverflowed(ss)
}

// sendHalted delivers table_halted (spec D15) to every subscriber of a
// table the run loop has stopped for good.
func (r *Registry) sendHalted(t *table, k int, reason string) {
	ss, _ := r.sessionsFor(t.cfg.ID)
	for _, s := range ss {
		s.push(frame(protocol.TTableHalted, t, k, 0, protocol.TableHaltedBody{Reason: reason}))
	}
	r.dropOverflowed(ss)
}

// archive writes the final sidecar, closes the files, and drops the
// engine and snapshots: a finished match is served from disk (spec:
// snapshots dropped when the match finishes). In memory mode the engine is
// kept so ViewAt still works.
func (r *Registry) archive(t *table, m *match) {
	if r.opts.Dir == "" {
		return
	}
	m.mu.Lock()
	sc := m.sidecar()
	m.files.close()
	m.files = nil
	m.snaps = nil
	m.mu.Unlock()
	if err := writeSidecar(r.opts.Dir, sc, r.opts.Sync); err != nil {
		m.mu.Lock()
		m.reason = "sidecar: " + err.Error()
		m.mu.Unlock()
	}
	t.mu.Lock()
	t.archived = append(t.archived, sc)
	t.mu.Unlock()
	r.mu.Lock()
	r.saveLocked()
	r.mu.Unlock()
}
