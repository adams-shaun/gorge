package azmcts

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
)

// Playout-cap randomization (PCR; KataGo, arXiv:1902.10565; DraftZero): at each
// of a seat's searched root decisions a seeded coin picks a FULL search (the
// configured one, recorded for training) or a CHEAP one (CheapSims, no root
// noise, the most-visited candidate, never recorded) that only moves the game
// on. SeatConfig.FullProb, CheapSims and FullProbByKind configure it; the
// seat applies it in rootOptions.

// pcrSalt separates the playout-cap draw from every stream Search derives from
// the same decision seed (its root rng, the environment bots' seed), so turning
// PCR on never perturbs a full search.
const pcrSalt = 0x7063722d66756c6c // "pcr-full"

// PCRFull is the playout-cap draw at a decision whose seed is decisionSeed
// (DecisionSeed(seat seed, decision sequence)): true -- a full search -- with
// probability p. It is a pure function of its arguments, so a game's full and
// cheap decisions never depend on the worker count or on other games. p >= 1
// is always full, p <= 0 never.
func PCRFull(decisionSeed uint64, p float64) bool {
	switch {
	case p >= 1:
		return true
	case !(p > 0): // also NaN
		return false
	}
	u := float64(splitmix(decisionSeed^pcrSalt)>>11) / (1 << 53)
	return u < p
}

// searchedKindNames is every searched kind's name (KindNames, then
// ExtraKindNames): the names :kinds= and FullProbByKind use, in a fixed
// order.
func searchedKindNames() []string {
	names := make([]string, 0, NumKinds+NumExtraKinds)
	return append(append(names, KindNames[:]...), ExtraKindNames[:]...)
}

// kindNameList is every searched kind's name, comma-separated (error texts).
func kindNameList() string { return strings.Join(searchedKindNames(), ", ") }

// validatePCR refuses playout-cap settings a seat must not run with: FullProb
// and every FullProbByKind entry in [0,1], per-kind entries only under
// FullProb > 0 and only for searched kinds' names, and CheapSims >= 1
// whenever a decision can be cheap. It walks the kind names in order, never
// the map, so the error it names is deterministic.
func (c *SeatConfig) validatePCR() error {
	if c.FullProb == 0 {
		if len(c.FullProbByKind) > 0 {
			return errors.New("azmcts: FullProbByKind needs FullProb > 0 (FullProb 0 turns playout-cap randomization off)")
		}
		return nil
	}
	if !(c.FullProb > 0 && c.FullProb <= 1) {
		return fmt.Errorf("azmcts: FullProb %g must be in [0,1] (0 is off)", c.FullProb)
	}
	cheap := c.FullProb < 1
	known := 0
	for _, k := range searchedKindNames() {
		p, ok := c.FullProbByKind[k]
		if !ok {
			continue
		}
		known++
		if !(p >= 0 && p <= 1) {
			return fmt.Errorf("azmcts: FullProbByKind[%s] %g must be in [0,1]", k, p)
		}
		cheap = cheap || p < 1
	}
	if known != len(c.FullProbByKind) {
		return fmt.Errorf("azmcts: FullProbByKind: a key is not a searched kind (want %s)", kindNameList())
	}
	if cheap && c.CheapSims < 1 {
		return fmt.Errorf("azmcts: CheapSims %d must be >= 1 when a decision can be cheap (FullProb %g)", c.CheapSims, c.FullProb)
	}
	return nil
}

// fullSearch is the playout-cap draw at the seat's decision d, whose seed is
// seed: always true with PCR off (FullProb 0), else PCRFull at d's kind's
// probability (FullProbByKind, else FullProb).
func (s *Seat) fullSearch(d *decision.Decision, seed uint64) bool {
	if s.cfg.FullProb <= 0 {
		return true
	}
	p := s.cfg.FullProb
	if len(s.cfg.FullProbByKind) > 0 {
		if q, ok := s.cfg.FullProbByKind[s.searchedKind(d)]; ok {
			p = q
		}
	}
	return PCRFull(seed, p)
}

// searchedKind is the name of d's kind as the seat's Search would search it
// (kindOf over Search.Kinds with Choose capped at Search.Limit, as Search
// caps it), "" when it searches none.
func (s *Seat) searchedKind(d *decision.Decision) string {
	kinds := s.cfg.Search.Kinds
	if kinds.Choose {
		kinds.chooseMax = s.cfg.Search.Limit
	}
	return kindOf(d, kinds)
}

// rootOptions are Search's options at the seat's own searched decision d,
// taken at game turn turn, and whether playout-cap randomization made it a
// cheap search. A full search (every search with PCR off) runs Search.Sims
// (d's kind's SimsByKind when set) with root noise and visit sampling as
// Explore says; a cheap one runs CheapSims with neither: no noise, the
// most-visited candidate. The seed is DecisionSeed either way.
func (s *Seat) rootOptions(d *decision.Decision, turn int32) (Options, bool) {
	opts := s.cfg.Search
	opts.Seed = DecisionSeed(s.seed, d.Seq)
	opts.Noise, opts.Sample = false, false
	if !s.fullSearch(d, opts.Seed) {
		opts.Sims = s.cfg.CheapSims
		return opts, true
	}
	if n := s.cfg.SimsByKind[d.Kind]; n > 0 {
		opts.Sims = n
	}
	if s.cfg.Explore {
		opts.Noise = !s.cfg.NoNoise
		opts.Sample = turn <= s.cfg.ExploreTurns
	}
	return opts, false
}

// PCRCounts are the seat's searched root decisions (a tree was built) that
// playout-cap randomization made full and cheap, both 0 with PCR off. A
// decided-game search (SeatConfig.DecidedHigh) counts among the full.
func (s *Seat) PCRCounts() (full, cheap int) { return s.pcrFull, s.pcrCheap }
