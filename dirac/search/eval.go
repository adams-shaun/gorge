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
// 1/(1+exp(-(mine-theirs)/20)). Uniform prior. With more than two seats
// theirs is the strongest living opponent's score, as gorge's
// heuristicLeafValue reads it (bots/azmcts env.go): a lost seat is skipped,
// a lost searching seat is 0 while the game plays on, and a game that is over
// is 1 for its winner, 0 for every other seat and 0.5 drawn.
type HeuristicLeaf struct{}

func (HeuristicLeaf) Value(s *dirac.State, _ *dirac.Decision, seat uint8) float32 {
	n := s.NumSeats()
	if n > 2 {
		if v, done := decidedValue(s, seat); done {
			return v
		}
	}
	score := heuristicScores(s)
	var v float64
	if n == 2 {
		v = score[seat] - score[1-seat]
	} else {
		v = score[seat] - maxOpponent(s, seat, &score)
	}
	return float32(1 / (1 + math.Exp(-v/20)))
}

// heuristicScores is every seat's HeuristicLeaf score: life + 2*hand +
// the battlefield's material it controls.
func heuristicScores(s *dirac.State) (score [dirac.MaxSeats]float64) {
	for p := uint8(0); p < s.NumSeats(); p++ {
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
	return score
}

// VecEvaluator is an optional Evaluator extension read under Options.MaxN
// (detected once in Searcher.hooks): ValueVec writes the position's value to
// every seat into out, indexed by the seat relative to seat in turn order
// (relSeat: 0 seat itself, 1 the next seat to take a turn, ...), each entry in
// [0,1] and a lost seat's 0. The search reads it only at a leaf of a game that
// is not over. An Evaluator without it values only seat's position, and the
// search splits the rest evenly over the living opponents (splitVec).
// Implementations must not allocate.
type VecEvaluator interface {
	ValueVec(s *dirac.State, d *dirac.Decision, seat uint8, out *[dirac.MaxSeats]float64)
}

// ValueVec is gorge's heuristicLeafVec (bots/azmcts env.go): a softmax over
// the living seats' scores at Value's temperature, exp(s_i/20) / sum
// exp(s_j/20), so with two living seats seat's entry is Value's sigmoid. A
// lost seat is 0.
func (HeuristicLeaf) ValueVec(s *dirac.State, _ *dirac.Decision, seat uint8, out *[dirac.MaxSeats]float64) {
	*out = [dirac.MaxSeats]float64{}
	n := s.NumSeats()
	score := heuristicScores(s)
	top, have := 0.0, false
	for p := uint8(0); p < n; p++ {
		if s.FeatLost(p) {
			continue
		}
		score[p] /= 20
		if !have || score[p] > top {
			top, have = score[p], true
		}
	}
	if !have {
		return
	}
	sum := 0.0
	for p := uint8(0); p < n; p++ {
		if s.FeatLost(p) {
			continue
		}
		x := math.Exp(score[p] - top)
		out[relSeat(p, seat, n)] = x
		sum += x
	}
	for r := range out {
		out[r] /= sum
	}
}

// relSeat is seat p relative to seat in turn order among n seats (0 seat
// itself, 1 the next seat to take a turn, ...): the index of p's entry in a
// value vector (Options.MaxN).
func relSeat(p, seat, n uint8) uint8 { return (p + n - seat) % n }

// decidedValue is a leaf's value to seat when the game has decided it: the
// game's end (1 to its winner, 0 to every other seat, 0.5 drawn), or seat's
// own loss while the game plays on for the other seats (CR 800.4a).
func decidedValue(s *dirac.State, seat uint8) (float32, bool) {
	if s.Over() {
		switch w := s.Winner(); {
		case w == int8(seat):
			return 1, true
		case w >= 0:
			return 0, true
		}
		return 0.5, true
	}
	if s.FeatLost(seat) {
		return 0, true
	}
	return 0, false
}

// maxOpponent is the largest score among seat's living opponents (0 when
// none lives).
func maxOpponent(s *dirac.State, seat uint8, score *[dirac.MaxSeats]float64) float64 {
	top, have := 0.0, false
	for p := uint8(0); p < s.NumSeats(); p++ {
		if p == seat || s.FeatLost(p) {
			continue
		}
		if !have || score[p] > top {
			top, have = score[p], true
		}
	}
	return top
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
