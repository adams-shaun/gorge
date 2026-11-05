package search

import (
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Evaluator supplies the search's leaf values and priors (azmcts: the
// policynet value head and prior; dirac/neteval, dirac/feat feeding
// gorge's bots/netsimd, plugs in here). Value is the win probability of seat in s, in [0,1].
// Value reads the position at decision d (the walk's pending decision).
// Priors writes a logit per option of d into out[:d.N]; bot[i] marks the
// options of the bot's own answer (policynet Option.BotPick). A candidate's
// logit is its option's (single kinds), policynet.CandidateScore's subset
// log-likelihood (attackers, blockers) or the sum of its picks' (fuller
// kinds); the prior is their softmax (azmcts priorsWith).
// The prior is used only by an Evaluator that also implements PriorSource
// with HasPrior true (a network); otherwise the prior is uniform and
// PriorTopK is off, as in azmcts without a net. Implementations must not
// allocate.
type Evaluator interface {
	Value(s *dirac.State, d *dirac.Decision, seat uint8) float32
	Priors(s *dirac.State, d *dirac.Decision, bot []bool, out []float32)
}

// BatchEvaluator is an optional Evaluator extension (detected once in
// Searcher.hooks, as WorldReset and BaseSetter are): it values len(states)
// positions in one call, out[i] the win probability of seat in states[i]
// at decs[i] (the walk's pending decision there). A Searcher with
// Options.Batch > 0 defers its descents' leaves and calls this instead of
// Value; an Evaluator without it keeps the serial path. Implementations
// must not allocate.
type BatchEvaluator interface {
	ValueBatch(states []*dirac.State, decs []*dirac.Decision, seat uint8, out []float32)
}

// Uniform is the zero-cost stub: value 0.5 (the uniform-prior,
// no-information leaf) and a uniform prior.
type Uniform struct{}

func (Uniform) Value(*dirac.State, *dirac.Decision, uint8) float32 { return 0.5 }

func (Uniform) ValueBatch(_ []*dirac.State, _ []*dirac.Decision, _ uint8, out []float32) {
	for i := range out {
		out[i] = 0.5
	}
}

func (Uniform) Priors(_ *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	for i := 0; i < int(d.N) && i < len(out); i++ {
		out[i] = 0
	}
}

// HeuristicLeaf is azmcts's generation-0 leaf (searchprobe.LeafValue):
// per player life + 2*hand + the battlefield's material (a land 3, any
// other permanent 10 + 2*(power+toughness) for a creature); the value is
// 1/(1+exp(-(mine-theirs)/20)). Uniform prior.
type HeuristicLeaf struct{}

func (HeuristicLeaf) Value(s *dirac.State, _ *dirac.Decision, seat uint8) float32 {
	var score [2]float64
	for p := uint8(0); p < 2; p++ {
		score[p] = float64(s.LifeOf(p) + 2*s.HandSize(p))
	}
	t := s.Table()
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjZone(ob) != kop.ZoneBattlefield {
			continue
		}
		p := s.Controller(ob)
		if t.Cards[s.ObjCard(ob)].Types&kop.TLand != 0 {
			score[p] += 3
			continue
		}
		m := 10.0
		if s.IsCreature(ob) {
			pw, tg := s.PowerToughness(ob)
			m += 2 * float64(pw+tg)
		}
		score[p] += m
	}
	v := score[seat] - score[1-seat]
	return float32(1 / (1 + math.Exp(-v/20)))
}

func (HeuristicLeaf) Priors(_ *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	for i := 0; i < int(d.N) && i < len(out); i++ {
		out[i] = 0
	}
}

func (h HeuristicLeaf) ValueBatch(states []*dirac.State, decs []*dirac.Decision, seat uint8, out []float32) {
	for i := range out {
		out[i] = h.Value(states[i], decs[i], seat)
	}
}
