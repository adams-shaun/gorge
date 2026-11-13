package search

import (
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// MZLeaf is MageZero's offline (gen0, no network) MCTS leaf, XMage
// GameStateEvaluator3 (WillWroble/mage a031818, Mage.Player.AI score/):
// per player 0.6*life (floored at 0) + the harmonic number of the hand
// size + 1 + mana value per permanent (a land 1, a token 1); the value is
// tanh((mine-theirs)/15) in [-1,1], mapped to a win probability
// (v+1)/2 (their backpropagate(1+score) on a [0,2] scale). A decided game
// is 1 or 0. Uniform prior. Their tree evaluates only priority nodes
// (other nodes inherit the parent's score); this leaf values whatever
// decision the walk stops at.
type MZLeaf struct{}

func (MZLeaf) Value(s *dirac.State, _ *dirac.Decision, seat uint8) float32 {
	if s.NumSeats() > 2 {
		return mzValueN(s, seat)
	}
	if s.Over() {
		switch s.Winner() {
		case int8(seat):
			return 1
		case int8(1 - seat):
			return 0
		}
		return 0.5
	}
	var perm [2]float64
	t := s.Table()
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjZone(ob) != kop.ZoneBattlefield {
			continue
		}
		perm[s.Controller(ob)] += 1 + float64(t.Cards[s.ObjCard(ob)].Cost.MV())
	}
	return mzValue(mzScore(s.LifeOf(seat), s.HandSize(seat), perm[seat]), mzScore(s.LifeOf(1-seat), s.HandSize(1-seat), perm[1-seat]))
}

// mzValueN is MZLeaf's value with more than two seats: theirs is the
// strongest living opponent's score, as HeuristicLeaf reads it at N seats.
func mzValueN(s *dirac.State, seat uint8) float32 {
	if v, done := decidedValue(s, seat); done {
		return v
	}
	var perm [dirac.MaxSeats]float64
	t := s.Table()
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjZone(ob) != kop.ZoneBattlefield {
			continue
		}
		perm[s.Controller(ob)] += 1 + float64(t.Cards[s.ObjCard(ob)].Cost.MV())
	}
	var score [dirac.MaxSeats]float64
	for p := uint8(0); p < s.NumSeats(); p++ {
		score[p] = mzScore(s.LifeOf(p), s.HandSize(p), perm[p])
	}
	return mzValue(score[seat], maxOpponent(s, seat, &score))
}

// mzScore is GameStateEvaluator3.evaluateResources: perm is the sum of
// 1 + mana value over the player's permanents.
func mzScore(life, hand int, perm float64) float64 {
	return 0.6*float64(max(0, life)) + harmonic(hand) + perm
}

// mzValue is evaluateNormalized's tanh on the [0,1] scale.
func mzValue(mine, theirs float64) float32 {
	return float32((math.Tanh((mine-theirs)/15) + 1) / 2)
}

func harmonic(n int) float64 {
	h := 0.0
	for i := 1; i <= n; i++ {
		h += 1 / float64(i)
	}
	return h
}

func (MZLeaf) Priors(_ *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	for i := 0; i < int(d.N) && i < len(out); i++ {
		out[i] = 0
	}
}

func (m MZLeaf) ValueBatch(states []*dirac.State, decs []*dirac.Decision, seat uint8, out []float32) {
	for i := range out {
		out[i] = m.Value(states[i], decs[i], seat)
	}
}
