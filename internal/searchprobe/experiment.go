package searchprobe

import (
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

type ExperimentOptions struct {
	Seed, SampleSeed             uint64
	Attempts, Worlds, MaxSubmits int
	// Clock is an optional diagnostic-only monotonic nanosecond source. Its
	// values are copied into result timing fields and never reach an engine,
	// proposal seed, budget, action, score, or control-flow decision. Keeping
	// the clock injected preserves this package's deterministic core boundary.
	Clock func() int64
}
type Outcome struct {
	Policy                 string
	Choice                 int
	Winner                 state.PlayerID
	Draw, Terminal, Replay bool
	Head                   string
	Submits                int
}
type ExperimentResult struct {
	Seed                uint64
	Actor               state.PlayerID
	RootAt              int
	RootTurn            int32
	MultiEpoch          bool
	Candidates          []Action
	Sampling            SampleResult
	OneWorld, FourWorld SearchResult
	StaticIndex         int
	Outcomes            []Outcome
	BaselineReplay      bool
	BaselineHead        string
	SampleNS, SearchNS  int64
	NoRootReason        string
	Unsupported, Error  string
}

// RunExperiment is the outcome harness, the only experiment component that
// owns actual secrets. Selection receives only the history and sampled worlds.
func RunExperiment(setup PublicGame, opts ExperimentOptions) (out ExperimentResult) {
	out.Seed = opts.Seed
	out.Actor = state.PlayerID(opts.Seed % 2)
	out.RootAt = -1
	cfg := rules.Config{Seed: opts.Seed, Names: setup.Names, Decks: setup.Decks, Tokens: setup.Tokens, StartingLife: setup.StartingLife}
	e := rules.New(cfg)
	e.Advance()
	rngs := BotRandoms(opts.Seed, len(setup.Names))
	board := botpolicy.NewBoard(len(rngs))
	collector := NewCollector(out.Actor)
	h := History{Actor: out.Actor, Answers: make(map[int][]Action)}
	var root *rules.Engine
	var samplingFallback string
	pos := 0
	for steps := 0; !e.G.Over; steps++ {
		if steps >= 20000 || e.G.Turn >= 200 {
			out.Error = "baseline stalled"
			return
		}
		d := e.Pending()
		if d == nil {
			out.Error = "baseline missing decision"
			return
		}
		in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
		if root == nil {
			f, err := collector.Capture(e, e.L.Events[pos:])
			if err != nil {
				var failure *Failure
				if !errors.As(err, &failure) || failure.Kind != "unsupported" {
					out.Error = "observation: " + err.Error()
					return
				}
				out.Unsupported = err.Error()
				// Retain this game's root and static/baseline comparisons even
				// when the prefix cannot be represented by this sampler.
				f, err = collector.Capture(e, nil)
				if err != nil {
					out.Error = err.Error()
					return
				}
			}
			h.Frames = append(h.Frames, f)
			if d.Player == out.Actor {
				a, err := collector.Actions(d, in)
				if err != nil {
					out.Error = err.Error()
					return
				}
				if e.G.Turn >= 5 && len(a) == 1 {
					out.Candidates = Candidates(f.Decision, a[0], 8)
				}
				if len(out.Candidates) > 0 {
					root = e.Clone()
					out.RootAt = steps
					out.RootTurn = e.G.Turn
					out.MultiEpoch = multiEpoch(h)
					start := diagnosticNow(opts.Clock)
					// SampleSeed is fixed for the experiment, never the game seed.
					if out.Unsupported == "" {
						out.Sampling, err = Sample(setup, h, SampleOptions{Seed: opts.SampleSeed, Attempts: opts.Attempts, Worlds: opts.Worlds, MaxSubmits: opts.MaxSubmits})
					}
					out.SampleNS = diagnosticSince(opts.Clock, start)
					if err != nil {
						var failure *Failure
						if errors.As(err, &failure) && (failure.Kind == "unsupported" || failure.Kind == "contradictory") {
							out.Sampling.Worlds = nil
							if failure.Kind == "unsupported" {
								out.Unsupported = err.Error()
							} else {
								samplingFallback = "contradictory public history: " + failure.Detail
							}
						} else {
							out.Error = "sampling: " + err.Error()
							return
						}
					}
					out.StaticIndex, err = StaticChoice(f, out.Candidates)
					if err != nil {
						out.Error = err.Error()
						return
					}
					start = diagnosticNow(opts.Clock)
					worlds := out.Sampling.Worlds
					out.FourWorld, err = SearchChoice(worlds, out.Candidates, opts.SampleSeed^0x474747, opts.MaxSubmits)
					if err != nil {
						out.Error = err.Error()
						return
					}
					if len(worlds) > 0 {
						worlds = worlds[:1]
					}
					out.OneWorld, err = SearchChoice(worlds, out.Candidates, opts.SampleSeed^0x474747, opts.MaxSubmits)
					out.SearchNS = diagnosticSince(opts.Clock, start)
					if err != nil {
						out.Error = err.Error()
						return
					}
					if samplingFallback != "" {
						out.OneWorld.Fallback, out.FourWorld.Fallback = samplingFallback, samplingFallback
					}
					// Do not retain large engines in the parallel result queue.
					out.Sampling.Worlds = nil
				} else {
					h.Answers[len(h.Frames)-1] = a
				}
			}
		}
		pos = len(e.L.Events)
		if err := e.Submit(in); err != nil {
			out.Error = err.Error()
			return
		}
	}
	out.BaselineHead = e.L.Head()
	if err := verifyActual(cfg, e); err != nil {
		out.Error = err.Error()
		return
	}
	out.BaselineReplay = true
	if root == nil {
		out.NoRootReason = "no eligible turn>=5 cast/ability/pass root"
		return
	}
	indices := []int{0, out.StaticIndex, out.OneWorld.Index, out.FourWorld.Index}
	policies := []string{"current", "static", "one-world", "four-world"}
	for i, index := range indices {
		in, err := collector.Match(root.Pending(), []Action{out.Candidates[index]})
		if err != nil {
			out.Error = err.Error()
			return
		}
		branch := root.Clone()
		if err := branch.Submit(in); err != nil {
			out.Error = err.Error()
			return
		}
		// Common independent continuation randomness for all actual branches.
		// The uninterrupted baseline above is separately audited.
		rngs := BotRandoms(opts.Seed^0xe7037ed1a0b428db, len(setup.Names))
		board := botpolicy.NewBoard(len(rngs))
		submits := 1
		for !branch.G.Over && submits < 20000 && branch.G.Turn < 200 {
			d := branch.Pending()
			if d == nil {
				out.Error = "outcome missing decision"
				return
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(branch.G, branch, d.Player, &board), d, rngs[d.Player])
			if err := branch.Submit(in); err != nil {
				out.Error = err.Error()
				return
			}
			submits++
		}
		o := Outcome{Policy: policies[i], Choice: index, Winner: branch.G.Winner, Draw: branch.G.Draw, Terminal: branch.G.Over, Head: branch.L.Head(), Submits: submits}
		if err := verifyActual(cfg, branch); err != nil {
			out.Error = err.Error()
			return
		}
		o.Replay = true
		out.Outcomes = append(out.Outcomes, o)
	}
	return
}

func diagnosticNow(clock func() int64) int64 {
	if clock == nil {
		return 0
	}
	return clock()
}

func diagnosticSince(clock func() int64, start int64) int64 {
	if clock == nil {
		return 0
	}
	return clock() - start
}

func verifyActual(cfg rules.Config, e *rules.Engine) error {
	replay := rules.New(cfg)
	replay.Advance()
	for _, in := range e.L.Intents {
		if err := replay.Submit(in); err != nil {
			return err
		}
	}
	if replay.L.Head() != e.L.Head() || replay.RNGDraws() != e.RNGDraws() || len(replay.L.Events) != len(e.L.Events) {
		return fmt.Errorf("actual outcome replay diverged")
	}
	return nil
}

func multiEpoch(h History) bool {
	shuffles := make(map[state.PlayerID]int)
	for _, f := range h.Frames {
		for _, ev := range f.Events {
			if ev.Kind == events.Shuffle {
				shuffles[ev.Player]++
				if shuffles[ev.Player] > 1 {
					return true
				}
			}
			if ev.Kind == events.LibraryOrder || ev.Kind == events.MoveZone && ev.To == state.ZLibrary {
				return true
			}
		}
	}
	return false
}
