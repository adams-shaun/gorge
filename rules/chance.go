package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
)

// ChanceDraw records one bounded random choice. It is an experimental replay
// input, not an event and not information that may be given to a playing seat.
type ChanceDraw struct {
	Bound int `json:"bound"`
	Value int `json:"value"`
}

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
	r.chance = &chanceState{prefix: append([]ChanceDraw(nil), prefix...)}
	e = newWithRNG(cfg, r)
	return e, nil
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
	prefix  []ChanceDraw
	draws   []ChanceDraw
	failure error
}

func (s *chanceState) clone() *chanceState {
	if s == nil {
		return nil
	}
	return &chanceState{
		prefix:  append([]ChanceDraw(nil), s.prefix...),
		draws:   append([]ChanceDraw(nil), s.draws...),
		failure: s.failure,
	}
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
	}
	s.draws = append(s.draws, ChanceDraw{Bound: n, Value: value})
	return value
}
