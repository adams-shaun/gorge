package host

import "github.com/adams-shaun/gorge/events"

// afterBurst runs inside the loop's Lock after a successful Submit: when
// the burst began a turn, the engine is cloned at this boundary (spec D11:
// turn-start snapshots, dropped with the match). Task 12 appends the
// burst to disk here too.
func (r *Registry) afterBurst(t *table, m *match, before int) error {
	if len(turnStartsIn(m.e.L.Events, before)) > 0 {
		m.snaps = append(m.snaps, snapshot{intent: m.intents, seq: m.bounds[len(m.bounds)-1] - 1, e: m.e.Clone()})
	}
	return r.persistBurst(t, m, before) // Task 12
}

// snapshotGenesis clones the engine at boundary 0, so a view inside turn 1
// never replays from scratch either.
func (m *match) snapshotGenesis() {
	m.snaps = append(m.snaps, snapshot{intent: 0, seq: m.bounds[0] - 1, e: m.e.Clone()})
}

// boundsOf derives the intent boundaries from the log alone: every burst
// but the last ends with the DecisionAsk of the next decision (Submit's
// handle/checkStateBased/Advance run until ask), so bounds[j] is one past
// the (j+1)-th DecisionAsk; a finished log's final burst ends at len. It
// equals the loop's own bookkeeping (viewat_test pins that) and lets a
// finished match served from files answer ViewAt without persisting bounds.
func boundsOf(evs []events.Event) []uint64 {
	var out []uint64
	for _, ev := range evs {
		if ev.Kind == events.DecisionAsk {
			out = append(out, ev.Seq+1)
		}
	}
	n := uint64(len(evs))
	if len(out) == 0 || out[len(out)-1] != n {
		out = append(out, n)
	}
	return out
}

func (r *Registry) persistBurst(t *table, m *match, before int) error { return nil } // Task 12 replaces
