package searchprobe

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// Search-teacher spike (2026-09-19). Ensemble determinization in the sense of
// Cowling, Ward & Powley (2012): the deciding seat's observation history is
// turned into K sampled worlds by Sample (which never sees the actual engine),
// every candidate answer is rolled out on every world with the default bot as
// the rollout policy for both seats, and the candidate with the best mean
// outcome is the label. Nothing here is wired into a production seat.

// TeacherOptions bounds one teacher call.
type TeacherOptions struct {
	// Seed fixes the rollout bot randomness; world i uses Seed^(i+1) for every
	// candidate (common random numbers across candidates).
	Seed uint64
	// HorizonTurns stops a rollout this many engine turns after the root and
	// scores the leaf with LeafValue. Zero rolls every rollout to game end.
	HorizonTurns int32
	// MaxSubmits caps each rollout; a capped rollout is scored as a leaf.
	MaxSubmits int
	// Margin is how much a candidate's mean value must exceed candidate 0's
	// (the default bot's answer) before the teacher overrides it.
	Margin float64
	// Clairvoyant marks worlds that are clones of the ACTUAL engine (hidden
	// zones and future chance included). It exists only to measure the ceiling
	// of rollout search at a decision kind; labels made this way cheat.
	Clairvoyant bool
	// Parallelism is how many goroutines run the rollouts (<=1: sequential).
	// Wall clock only: the result is identical whatever it is set to.
	Parallelism int
}

// TeacherResult reports per-candidate mean values in [0,1] from the deciding
// seat's point of view.
type TeacherResult struct {
	Index               int
	Values              []float64
	Rollouts, Submits   int
	Terminal, Capped    int
	WinsByCandidate     []int
	TerminalByCandidate []int
}

// LeafValue squashes the frozen LeafScore into (0,1). Terminal states map to
// 1 (win), 0 (loss), 0.5 (draw). The /20 scale is untuned: roughly one
// creature of material or 20 life moves the value from 0.5 to 0.73.
func LeafValue(v view.View, actor state.PlayerID) float64 {
	if v.Over {
		switch {
		case v.Draw:
			return 0.5
		case v.Winner != nil && *v.Winner == actor:
			return 1
		default:
			return 0
		}
	}
	return 1 / (1 + math.Exp(-LeafScore(v, actor)/20))
}

// TeacherChoice rolls every candidate on every world. candidates[0] must be
// the default bot's answer; each candidate is the full semantic answer (a
// multi-select attackers declaration is several Actions).
//
// A panic inside a rollout is returned as an error (the caller falls back to
// the bot). Measured cause on 2026-09-19: rules/livelock.go detect() indexes
// sigAt(j-p) below zero while a Clone's freshly reset watcher window is short
// and its signatures repeat.
func TeacherChoice(worlds []World, candidates [][]Action, opts TeacherOptions) (res TeacherResult, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("rollout panic: %v", p)
		}
	}()
	res = TeacherResult{Values: make([]float64, len(candidates)), WinsByCandidate: make([]int, len(candidates)), TerminalByCandidate: make([]int, len(candidates))}
	if len(candidates) < 2 || len(worlds) == 0 || opts.MaxSubmits < 1 {
		return res, fmt.Errorf("teacher needs >=2 candidates, >=1 world and a submit budget")
	}
	// One rollout per (world, candidate). Each is a pure function of its own
	// engine clone, its candidate and the world's seed, so the rollouts may
	// run on several goroutines; the clones are taken up front on the caller's
	// goroutine (Clone reads the shared source engine) and the outcomes are
	// folded in the sequential loop's own (world, candidate) order, so every
	// float sum and the first error are exactly the sequential ones.
	type rollout struct {
		submits   int
		over, won bool
		capped    bool
		value     float64
		err       error
		panicked  any
		done      bool
	}
	outs := make([]rollout, len(worlds)*len(candidates))
	run := func(k int, e *rules.Engine) {
		o := &outs[k]
		defer func() {
			if p := recover(); p != nil {
				o.panicked = p
			}
		}()
		o.done = true
		wi, ci := k/len(candidates), k%len(candidates)
		w, cand := worlds[wi], candidates[ci]
		d := e.Pending()
		if d == nil {
			o.err = fmt.Errorf("world has no pending root decision")
			return
		}
		in, err := w.Observer.Match(d, cand)
		if err != nil {
			o.err = fmt.Errorf("candidate %d does not map into world %d: %w", ci, wi, err)
			return
		}
		actor, turn := d.Player, e.G.Turn
		submit := e.SubmitHypothetical
		if opts.Clairvoyant {
			submit = e.Submit
		}
		if o.err = submit(in); o.err != nil {
			return
		}
		o.submits = 1
		rngs := BotRandoms(opts.Seed^uint64(wi+1), len(e.G.Players))
		board := botpolicy.NewBoard(len(rngs))
		for !e.G.Over && o.submits < opts.MaxSubmits && (opts.HorizonTurns == 0 || e.G.Turn < turn+opts.HorizonTurns) {
			pd := e.Pending()
			if pd == nil {
				o.err = fmt.Errorf("rollout has no pending decision")
				return
			}
			rin := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, pd.Player, &board), pd, rngs[pd.Player])
			if o.err = submit(rin); o.err != nil {
				return
			}
			o.submits++
		}
		o.over = e.G.Over
		o.won = e.G.Over && !e.G.Draw && e.G.Winner == actor
		o.capped = !e.G.Over && o.submits >= opts.MaxSubmits
		o.value = LeafValue(view.Project(e.G, e, actor, e.Pending()), actor)
	}
	if workers := min(opts.Parallelism, len(outs)); workers > 1 {
		engines := make([]*rules.Engine, len(outs))
		for k := range engines {
			engines[k] = worlds[k/len(candidates)].Engine.Clone()
		}
		var wg sync.WaitGroup
		var next atomic.Int64
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					k := int(next.Add(1)) - 1
					if k >= len(outs) {
						return
					}
					run(k, engines[k])
					engines[k] = nil
				}
			}()
		}
		wg.Wait()
	} else {
		for k := range outs {
			run(k, worlds[k/len(candidates)].Engine.Clone())
			if outs[k].err != nil || outs[k].panicked != nil {
				break
			}
		}
	}
	for k := range outs {
		o := &outs[k]
		if o.panicked != nil {
			panic(o.panicked) // recovered into err by the deferred handler above
		}
		if !o.done {
			break
		}
		if o.err != nil {
			return res, o.err
		}
		ci := k % len(candidates)
		res.Rollouts++
		res.Submits += o.submits
		if o.over {
			res.Terminal++
			res.TerminalByCandidate[ci]++
			if o.won {
				res.WinsByCandidate[ci]++
			}
		} else if o.capped {
			res.Capped++
		}
		res.Values[ci] += o.value / float64(len(worlds))
	}
	best := 0
	for i := 1; i < len(res.Values); i++ {
		if res.Values[i] > res.Values[best] {
			best = i
		}
	}
	if best != 0 && res.Values[best] <= res.Values[0]+opts.Margin {
		best = 0
	}
	res.Index = best
	return res, nil
}

// AttackCandidates enumerates declarations to compare at a KAttackers root:
// the bot's answer first, then no attack, all-in, and each single-attacker
// toggle of the bot's answer, deduplicated and validated, capped at limit.
// It assumes one option per attacker (two seats); options sharing an
// attacker keep only the first defender for the all-in candidate.
func AttackCandidates(d *decision.Decision, bot decision.Intent, limit int) []decision.Intent {
	if d == nil || d.Kind != decision.KAttackers || limit < 2 {
		return nil
	}
	chosen := make(map[int]bool, len(bot.Choices))
	for _, c := range bot.Choices {
		chosen[c] = true
	}
	var out []decision.Intent
	seen := make(map[string]bool)
	add := func(set map[int]bool) {
		if len(out) >= limit {
			return
		}
		var choices []int
		for _, o := range d.Options {
			// Decision.Validate does not enforce Required; the engine does
			// (CR 508.1d), so never offer a declaration omitting one.
			if o.Required && !set[o.Index] {
				return
			}
			if set[o.Index] {
				choices = append(choices, o.Index)
			}
		}
		key := fmt.Sprint(choices)
		if seen[key] {
			return
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
		if d.Validate(in) != nil {
			return
		}
		seen[key] = true
		out = append(out, in)
	}
	add(chosen)
	if len(out) == 0 {
		return nil
	}
	none := make(map[int]bool)
	for _, o := range d.Options {
		if o.Required {
			none[o.Index] = true
		}
	}
	add(none)
	all := make(map[int]bool)
	attacking := make(map[state.ObjID]bool)
	for _, o := range d.Options {
		if !attacking[o.Obj] {
			attacking[o.Obj] = true
			all[o.Index] = true
		}
	}
	add(all)
	for _, o := range d.Options {
		t := make(map[int]bool, len(chosen)+1)
		for k := range chosen {
			t[k] = true
		}
		if t[o.Index] {
			delete(t, o.Index)
		} else {
			t[o.Index] = true
		}
		add(t)
	}
	if len(out) < 2 {
		return nil
	}
	return out
}
