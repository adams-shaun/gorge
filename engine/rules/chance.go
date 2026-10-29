package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// ChanceDraw records one bounded random choice. It is an experimental replay
// input, not an event and not information that may be given to a playing seat.
type ChanceDraw struct {
	Bound int `json:"bound"`
	Value int `json:"value"`
}

type ShuffleCard struct {
	ID   state.ObjID
	Name string
}

type ShuffleContext struct {
	Player        state.PlayerID
	Ordinal       int
	Library, Hand []ShuffleCard
}

type ShufflePlanner func(ShuffleContext) ([]state.ObjID, error)

// ChanceContext is one non-shuffle draw past the chance prefix: Ordinal counts
// the planner's own draws from zero, and Bound is the requested IntN bound.
type ChanceContext struct {
	Ordinal int
	Bound   int
}

// ChancePlanner supplies one non-shuffle draw. The value must be in
// [0, ctx.Bound); a planner error or an out-of-range value fails the engine's
// chance boundary like any other chance failure.
type ChancePlanner func(ChanceContext) (int, error)

// NewHypothetical constructs an independent engine using checked chance
// outcomes from prefix, then cfg.Seed's generator. Callers must supply their
// own hypothetical seed and deck configuration, never the actual game's secret
// seed or hidden zones. Each forced draw also advances the seeded generator;
// an empty prefix therefore produces exactly New(cfg)'s ordinary events.
//
// All generated draws are recorded by ChanceTranscript. Replaying a hypothesis
// requires its Config, full chance transcript, and intents together; the normal
// replay package intentionally continues to implement Config-only randomness.
// If construction fails, no usable engine is returned.
func NewHypothetical(cfg Config, prefix []ChanceDraw) (e *Engine, err error) {
	return NewHypotheticalPlanned(cfg, prefix, nil)
}

// NewHypotheticalPlanned is NewHypothetical with an opt-in callback at real
// library-shuffle sites. The callback sees only this hypothetical game's
// cards. Its chosen permutation is encoded into the ordinary chance transcript,
// so replay needs no callback.
func NewHypotheticalPlanned(cfg Config, prefix []ChanceDraw, planner ShufflePlanner) (e *Engine, err error) {
	return NewHypotheticalChancePlanned(cfg, prefix, planner, nil)
}

// NewHypotheticalChancePlanned is NewHypotheticalPlanned with an opt-in
// callback for every non-shuffle draw past the prefix. Its value is recorded
// in the chance transcript exactly like a prefix value, so replay needs no
// callback. A nil chance planner leaves such draws unplanned, as before.
func NewHypotheticalChancePlanned(cfg Config, prefix []ChanceDraw, planner ShufflePlanner, chance ChancePlanner) (e *Engine, err error) {
	for i, d := range prefix {
		if d.Bound <= 0 {
			return nil, fmt.Errorf("hypothetical chance draw %d: invalid bound %d", i, d.Bound)
		}
		if d.Value < 0 || d.Value >= d.Bound {
			return nil, fmt.Errorf("hypothetical chance draw %d: value %d outside [0,%d)", i, d.Value, d.Bound)
		}
	}
	defer recoverChance(&err)
	r := newRNG(cfg.Seed)
	r.chance = &chanceState{
		prefix:          append([]ChanceDraw(nil), prefix...),
		planner:         planner,
		chancePlanner:   chance,
		shuffleOrdinals: make(map[state.PlayerID]int),
	}
	e = newWithRNG(cfg, r, false)
	return e, nil
}

// Hypothetical reports whether the engine carries a chance boundary, so a
// caller can choose SubmitHypothetical over Submit on a clone.
func (e *Engine) Hypothetical() bool {
	return e != nil && e.rng != nil && e.rng.chance != nil
}

// SubmitHypothetical submits an intent and reports an incompatible chance
// prefix as an error. A chance failure may happen mid-resolution: the engine
// is then unusable and must be discarded, never repaired or used for scoring.
// Subsequent calls return the same failure without mutating it further.
// This boundary catches only chance failures; unrelated engine panics propagate.
func (e *Engine) SubmitHypothetical(in decision.Intent) (err error) {
	if e == nil || e.rng == nil || e.rng.chance == nil {
		return fmt.Errorf("SubmitHypothetical requires a hypothetical engine")
	}
	if e.rng.chance.failure != nil {
		return e.rng.chance.failure
	}
	defer recoverChance(&err)
	return e.Submit(in)
}

// AdvanceHypothetical is Advance with the same chance-error boundary as
// SubmitHypothetical. Like New, NewHypothetical does not automatically advance
// to a decision; callers use this method before reading the first Pending.
func (e *Engine) AdvanceHypothetical() (err error) {
	if e == nil || e.rng == nil || e.rng.chance == nil {
		return fmt.Errorf("AdvanceHypothetical requires a hypothetical engine")
	}
	if e.rng.chance.failure != nil {
		return e.rng.chance.failure
	}
	defer recoverChance(&err)
	e.Advance()
	return nil
}

// ChanceTranscript returns an owned copy of the outcomes consumed so far.
// Ordinary engines do not record a transcript and return nil.
func (e *Engine) ChanceTranscript() []ChanceDraw {
	if e == nil || e.rng == nil || e.rng.chance == nil {
		return nil
	}
	return append([]ChanceDraw(nil), e.rng.chance.draws...)
}

type chanceState struct {
	prefix          []ChanceDraw
	draws           []ChanceDraw
	failure         error
	planner         ShufflePlanner
	chancePlanner   ChancePlanner
	shuffleOrdinals map[state.PlayerID]int
	// nonShuffle counts the draws chancePlanner has supplied, so its ordinal
	// counts only non-shuffle draws from zero.
	nonShuffle int
	// inShuffle marks the draws forcePermutation consumes as a planned
	// library shuffle; they are not non-shuffle chance draws.
	inShuffle bool
	// unplanned counts draws recorded from the engine's own generator. It is
	// a counter, not len(draws)-len(prefix): a planned shuffle materializes
	// the already-generated suffix into prefix (rng.forcePermutation), so the
	// lengths alone would retroactively claim those earlier draws were
	// supplied by a planner.
	unplanned int
}

func (s *chanceState) clone() *chanceState {
	if s == nil {
		return nil
	}
	return &chanceState{
		prefix:     append([]ChanceDraw(nil), s.prefix...),
		draws:      append([]ChanceDraw(nil), s.draws...),
		failure:    s.failure,
		nonShuffle: s.nonShuffle,
		unplanned:  s.unplanned,
	}
}

// UnplannedDraws reports how many of a hypothetical engine's recorded draws no
// planner or prefix supplied. The count is taken when each draw is recorded,
// so a later planned shuffle cannot reclassify an earlier unplanned draw. A
// restore rewinds it with the transcript, so a replayed lineage reports its
// own history, not the sum of every aborted re-run.
func (e *Engine) UnplannedDraws() int {
	if e == nil || e.rng == nil || e.rng.chance == nil {
		return 0
	}
	return e.rng.chance.unplanned
}

// ClearHypotheticalPlanner ends proposal construction. Future chance uses the
// hypothesis's independently seeded generator while the recorded prefix stays
// available for replay.
func (e *Engine) ClearHypotheticalPlanner() {
	if e != nil && e.rng != nil && e.rng.chance != nil {
		e.rng.chance.planner = nil
		e.rng.chance.chancePlanner = nil
		e.rng.chance.shuffleOrdinals = nil
	}
}

func (e *Engine) shuffleCards(ids []state.ObjID) []ShuffleCard {
	out := make([]ShuffleCard, 0, len(ids))
	for _, id := range ids {
		name := ""
		if obj := e.G.Obj(id); obj != nil && obj.Card != nil && len(obj.Card.Faces) > 0 {
			name = obj.Card.Faces[0].Name
		}
		out = append(out, ShuffleCard{ID: id, Name: name})
	}
	return out
}

func (s *chanceState) fail(err error) {
	if s.failure == nil {
		s.failure = err
	}
	panic(chanceFailure{s.failure})
}

type chanceFailure struct{ err error }

func recoverChance(err *error) {
	if p := recover(); p != nil {
		if failure, ok := p.(chanceFailure); ok {
			*err = failure.err
			return
		}
		panic(p)
	}
}

func (s *chanceState) check(n int) {
	if s.failure != nil {
		panic(chanceFailure{s.failure})
	}
	if i := len(s.draws); i < len(s.prefix) && s.prefix[i].Bound != n {
		s.failure = fmt.Errorf("hypothetical chance draw %d: bound %d, engine requested %d", i, s.prefix[i].Bound, n)
		panic(chanceFailure{s.failure})
	}
}

func (s *chanceState) record(n, value int) int {
	if i := len(s.draws); i < len(s.prefix) {
		value = s.prefix[i].Value
	} else if s.chancePlanner != nil && !s.inShuffle {
		v, err := s.chancePlanner(ChanceContext{Ordinal: s.nonShuffle, Bound: n})
		if err != nil {
			s.fail(fmt.Errorf("hypothetical chance planner draw %d: %w", len(s.draws), err))
		}
		if v < 0 || v >= n {
			s.fail(fmt.Errorf("hypothetical chance planner draw %d: value %d outside [0,%d)", len(s.draws), v, n))
		}
		s.nonShuffle++
		value = v
	} else {
		s.unplanned++
	}
	s.draws = append(s.draws, ChanceDraw{Bound: n, Value: value})
	return value
}

// CloneHypothetical returns an independent copy of e (Clone) whose future
// chance is a fresh generator seeded by seed, recorded like any hypothetical
// engine's so SubmitHypothetical accepts it. Nothing of e's own generator
// position is carried over: a search world built from an actual engine must
// not inherit that game's future random outcomes. The copy's transcript
// starts empty, so it is not replayable from Config; it is a search world,
// never a match. e itself is not changed.
func (e *Engine) CloneHypothetical(seed uint64) *Engine {
	return e.CloneHypotheticalInto(seed, nil)
}

// CloneHypotheticalInto is CloneHypothetical drawing the copy's arrays from
// *sp exactly as CloneInto does (nil or a zero Spare: plain
// CloneHypothetical). The search loop releases each simulation's world back
// into the Spare the next one is built from.
func (e *Engine) CloneHypotheticalInto(seed uint64, sp *Spare) *Engine {
	c := e.CloneInto(sp)
	r := newRNG(seed)
	r.chance = &chanceState{}
	c.rng = r
	if c.tape.Posed() {
		// A world forked at a posed tape resolution re-runs that resolution
		// from S0, which holds the TRUE hidden cards and RNG: give the world
		// its own checkpoint copy whose re-run draws S0's values only up to
		// the fork and the world's own after it, and re-applies the world's
		// redeal at the fork point (lasagna spec §7.3).
		c.tape.Fork(e.forkSplice(c.rng), c.L)
	}
	c.rng.forked = true
	return c
}

// forkSplice is the RNG splice of a world forked from e at e's posed tape
// resolution: the world's re-run from S0 draws what e's own re-run would up
// to the fork, then next's values.
//
// When e is itself a world forked at this resolution (a redeal of a redeal),
// its draw counter is not S0's: it continues S0's only once e has re-run.
// Unre-run, e's re-run would switch generators at its own splice point and
// draw nothing more before the posed decision, so the new world switches
// there too. Re-run, e drew its own generator from its splice point to now,
// so the new world's chain follows e's and switches at e's current count.
func (e *Engine) forkSplice(next *rng) *rngSplice {
	prev, _ := e.tape.PosedWorld().(*rngSplice)
	switch {
	case prev == nil:
		return &rngSplice{at: e.rng.Draws, next: next.clone()}
	case e.rng.forked:
		return &rngSplice{at: prev.at, next: next.clone()}
	default:
		return prev.then(e.rng.Draws, next.clone())
	}
}
