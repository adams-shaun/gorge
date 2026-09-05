package host

import "github.com/adams-shaun/gorge/events"

// afterBurst runs inside the loop's Lock after a successful Submit: when
// the burst began a turn, the engine is cloned at this boundary (spec D11:
// turn-start snapshots, dropped with the match). Task 12 appends the
// burst to disk here too.
//
// FL-44: snapshots retain the full log prefix; see ledger. Engine.Clone
// deep-copies G, L (the whole recorded log so far) and the trigger/
// continuous-effect bookkeeping, so every stored snapshot is roughly as
// big as the match was at that point — measured ~5-8 MB retained per
// 4-seat match's worth of snapshots, unbounded on t.history. A follow-up
// (slim snapshots keeping only the chain head + counts, finished matches
// shedding snapshots once persisted) is dispatched separately; this task
// takes the straightforward Clone the brief specifies.
func (r *Registry) afterBurst(t *table, m *match, before int) error {
	if len(turnStartsIn(m.e.L.Events, before)) > 0 {
		m.snaps = append(m.snaps, snapshot{intent: m.intents, seq: m.bounds[len(m.bounds)-1] - 1, e: m.e.Clone()})
	}
	return r.persistBurst(t, m, before) // Task 12
}

// snapshotGenesis clones the engine at boundary 0, so a view inside turn 1
// never replays from scratch either. FL-44 (see afterBurst) applies here
// too.
func (m *match) snapshotGenesis() {
	m.snaps = append(m.snaps, snapshot{intent: 0, seq: m.bounds[0] - 1, e: m.e.Clone()})
}

// boundsOf derives the intent boundaries from the log alone: every burst
// but the last ends with the DecisionAsk of the next decision (Submit's
// handle/checkStateBased/Advance run until ask), so bounds[j] is one past
// the (j+1)-th DecisionAsk. It equals the loop's own bookkeeping
// (viewat_test pins that) and lets a finished match served from files
// answer ViewAt without persisting bounds.
//
// A tail past the last DecisionAsk is a real, complete final boundary only
// when it ends in GameOver: Advance stops asking once the game is over, so
// a naturally finished log's last burst has no trailing ask (fix round 1,
// FL-43 — this used to synthesize that final boundary unconditionally).
// Anything else back there is a partial burst: Submit records an intent
// into L.Intents before handle/checkStateBased/Advance run, so a panic
// during that processing (a crashed match, spec D15) leaves a straggler
// tail with no ask and no GameOver — real logged events, but an
// incomplete one whose owning intent never finished. Treating that as a
// boundary would make viewAt resubmit the very intent that crashed the
// engine, in the reader's own goroutine. Leaving it unbounded means such a
// tail is served the other way viewAt already handles intra-burst seqs:
// from the previous real boundary, by events.Apply of exactly what was
// logged, never by re-running Submit.
func boundsOf(evs []events.Event) []uint64 {
	var out []uint64
	for _, ev := range evs {
		if ev.Kind == events.DecisionAsk {
			out = append(out, ev.Seq+1)
		}
	}
	n := uint64(len(evs))
	if (len(out) == 0 || out[len(out)-1] != n) && n > 0 && evs[n-1].Kind == events.GameOver {
		out = append(out, n)
	}
	return out
}

// persistBurst appends the burst and its intent (PL-2). Called with m.mu
// held; an error propagates through afterBurst to play, which crashes the
// match (D15) — a table whose files cannot be written must not keep going.
func (r *Registry) persistBurst(t *table, m *match, before int) error {
	if m.files == nil {
		return nil
	}
	in := m.e.L.Intents[len(m.e.L.Intents)-1]
	return m.files.append(m.e.L.Events[before:], &in)
}
