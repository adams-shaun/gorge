package search

// Early cut (the :earlycut=<rule> spec key): a decision's search
// stops before Options.Sims simulations when the rest of the budget cannot
// change the move, or is unlikely to.
//
//   - EarlyCutGap: the leader's visits minus the runner-up's exceed the
//     simulations left. Every simulation adds at most one visit to one
//     root candidate, so the runner-up cannot reach the leader: the argmax
//     pick (ties to the lower index) is the one the full budget makes, and
//     an eval-mode game is unchanged move for move.
//   - EarlyCutShare > 0: after at least EarlyCutMin simulations the
//     leader holds more than that share of the root candidates' visits.
//     A heuristic: the full budget may have picked another move.
//
// Both read the root point's candidates only (the first rootPt.n edges of
// the root node, the ones Result reports); a world's own root keys
// appended after them never become the choice. The cut applies to the
// fresh-deal search (CachedWorlds == 0) only. Under Sample (self-play
// exploration) the visit-proportional pick and the recorded visit counts
// come from the shorter search. Off (the zero value) by default.

// CutTrace records a search's root after every simulation (fresh-deal
// searches only), so every early-cut rule can be scored against the full
// search offline: the cut search is a prefix of the full one. Set
// Searcher.Trace with buffers of at least Options.Sims entries; Search
// fills N entries (N = the simulations run).
type CutTrace struct {
	N                int
	Lead, Run, Total []int32 // leader's and runner-up's visits, root visits
	Arg              []uint8 // the argmax candidate (ties to the lower index)
}

// NewCutTrace sizes a trace for sims simulations.
func NewCutTrace(sims int) *CutTrace {
	return &CutTrace{Lead: make([]int32, sims), Run: make([]int32, sims), Total: make([]int32, sims), Arg: make([]uint8, sims)}
}

// earlyCut reports whether the search may stop after done of sims
// simulations (and records the root in Trace).
func (sr *Searcher) earlyCut(done, sims int) bool {
	t := &sr.t
	if t.nn == 0 {
		return false
	}
	var lead, run, total int32
	lead, run = -1, -1
	arg, i := 0, 0
	for e := t.nodes[0].first; e >= 0 && i < sr.rootPt.n; e = t.edges[e].sib {
		n := t.edges[e].n
		total += n
		if n > lead {
			lead, run, arg = n, lead, i
		} else if n > run {
			run = n
		}
		i++
	}
	if tr := sr.Trace; tr != nil && done-1 < len(tr.Lead) {
		tr.N = done
		tr.Lead[done-1], tr.Run[done-1], tr.Total[done-1], tr.Arg[done-1] = lead, max(run, 0), total, uint8(arg)
	}
	if i < 2 || done >= sims {
		return false
	}
	return CutRule{Gap: sr.Opts.EarlyCutGap, Share: sr.Opts.EarlyCutShare, Min: sr.Opts.EarlyCutMin}.Fires(lead, max(run, 0), total, done, sims)
}

// CutRule is one early-cut rule (Options.EarlyCut*).
type CutRule struct {
	Gap   bool
	Share float64
	Min   int
}

// Fires reports whether the rule stops a search after done of sims
// simulations with the root's leader at lead visits, the runner-up at run
// and total root visits.
func (c CutRule) Fires(lead, run, total int32, done, sims int) bool {
	if c.Gap && int(lead-run) > sims-done {
		return true
	}
	return c.Share > 0 && done >= c.Min && total > 0 && float64(lead) > c.Share*float64(total)
}
