package host

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
)

// afterBurst runs inside the loop's Lock after a successful Submit: when
// the burst began a turn, the engine is cloned at this boundary (spec D11:
// turn-start snapshots, dropped with the match). Task 12 appends the
// burst to disk here too, and Task M2c-1 delivers it to the embedder's
// OnBurst hook.
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
	if err := r.persistBurst(t, m, before); err != nil { // Task 12
		return err
	}
	return r.observeBurst(t, m, before) // Task M2c-1
}

// observeBurst delivers the burst that just ended to the embedder's OnBurst
// hook (Task M2c-1, FL-81), if one is set. Called with m.mu held, on the
// match goroutine — after the burst has been persisted (so a replay served
// from the hook is never ahead of what is durable) but before the match
// loop continues, and with before == 0 it is how the genesis burst (no
// intent yet) is delivered from play(). An OnBurst error crashes the match
// exactly as a persist failure does (D15): the caller propagates it and the
// match stops rather than continuing silently.
//
// evs is copied (a fresh backing array) so the burst slice cannot alias the
// live engine log the loop keeps appending to; the events' IDs/Pairs and in
// are still read-only references into the live match, which the callback
// must not mutate or retain beyond its own copy (see OnBurstFunc's doc).
func (r *Registry) observeBurst(t *table, m *match, before int) error {
	if r.opts.OnBurst == nil {
		return nil
	}
	var in *decision.Intent
	if n := len(m.e.L.Intents); n > 0 {
		in = &m.e.L.Intents[n-1]
	}
	evs := append([]events.Event(nil), m.e.L.Events[before:]...)
	return r.opts.OnBurst(t.cfg.ID, m.k, evs, in)
}

// observeMatchEnd delivers a match's terminal MatchInfo to the embedder's
// OnMatchEnd hook (Task M2c-1), if one is set. Called from finish/abort/
// crash on the match goroutine after the final state is recorded; the
// hook's error cannot change that already-decided outcome, so it is
// surfaced only by the embedder itself (see OnMatchEndFunc). A match
// rewritten aborted during a restart (host/restart.go) goes through
// callOnMatchEnd with a sidecar-derived MatchInfo instead.
func (r *Registry) observeMatchEnd(t *table, m *match) {
	if r.opts.OnMatchEnd == nil {
		return
	}
	m.mu.RLock()
	info := m.info()
	m.mu.RUnlock()
	r.callOnMatchEnd(t.cfg.ID, m.k, info)
}

// callOnMatchEnd is the single place OnMatchEnd actually fires, shared by
// observeMatchEnd (a live match's own terminal transition) and load() (an
// archived match rewritten aborted on restart). The error return is ignored
// here — see observeMatchEnd's and OnMatchEndFunc's docs.
func (r *Registry) callOnMatchEnd(t TableID, k int, m protocol.MatchInfo) {
	if r.opts.OnMatchEnd == nil {
		return
	}
	_ = r.opts.OnMatchEnd(t, k, m)
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
// m.persisted advances only on a fully-appended burst, so it is always the
// count of a complete, durable prefix (fix round 1).
func (r *Registry) persistBurst(t *table, m *match, before int) error {
	if m.files == nil {
		return nil
	}
	in := m.e.L.Intents[len(m.e.L.Intents)-1]
	if err := m.files.append(m.e.L.Events[before:], &in); err != nil {
		return err
	}
	m.persisted = len(m.e.L.Events)
	return nil
}
