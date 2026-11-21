package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A SOLE life-only mana ability (Treasonous Ogre's "Pay 3 life: Add {R}",
// kop.SoleGapLifeManaAb) is the planner's last-resort unit, not a census gap
// (mana_lifegap.go), but gorge's prefix search lists it as an "activate"
// option at every node, beside the covered sources that fund a filter's fee.
// It taps nothing, so it can pay a fee again after the plan paid the life,
// and the same source funds the fee and then, as the plan's unit, a pip of the
// play (Edgar Markov's {3}{R}{W}{B} beside Artifact Unknown Shores, three
// Swamps, a Mountain and the Ogre: the Ogre's {R} pays the Shores' {1} for
// {W}, the lands pay four pips and the Ogre pays the last, CR 605.3a, 119.4).
// The step is a funding one (filtStep.soleLife): it pays the life, adds the
// mana and leaves the source untapped.

// soleLifeSource returns sl's sole life-only mana ability when it funds a fee
// of p now: a fixed colour (an any-colour one asks, which the search does not
// model), not lost, and payable (a payment that ends the game is no node of
// the search, filtSearch.try).
func (s *State) soleLifeSource(k *prep, p uint8, sl Slot) (*kop.Ability, bool) {
	if k.gen == nil || !k.painMana || s.genLost(k, sl) {
		return nil, false
	}
	pc := s.manaPCGen(k, sl)
	if !lifeOnlyPain(pc) {
		return nil, false
	}
	a := &k.t.Cards[s.cardAt(sl)].Abilities[pc.manaAb]
	if !kop.GapLifeManaAb(a) || a.ManaColors == kop.ColAny || int(s.pl[SeatIx(p)].life) < lifeCostOwed(&a.Cost) {
		return nil, false
	}
	return a, true
}

// soleLifeFundDo is the funding step: the life is paid, the mana floats.
func (s *State) soleLifeFundDo(k *prep, p uint8, sl Slot) {
	a, ok := s.soleLifeSource(k, p, sl)
	if !ok {
		return
	}
	s.payLifeCost(k, p, &a.Cost, 0)
	col := bits.TrailingZeros8(uint8(a.ManaColors))
	s.pl[SeatIx(p)].pool[col] += max(a.ManaN, 1)
	s.artTag(k, p, sl, col)
}

// fundNoTap tries sl as a funding source that taps nothing: a sacrifice-only
// one (an Eldrazi Spawn, mana_filtfund.go) or a sole life-only one.
func (fs *filtSearch) fundNoTap(k *prep, t *State, sl Slot, funded int, gapUsed bool) bool {
	if t.canSacOnlyNowAt(k, sl) && fs.try(k, t, filtStep{sl: sl, sacOnly: true}, funded+1, gapUsed) {
		return true
	}
	if _, ok := t.soleLifeSource(k, fs.p, sl); ok {
		return fs.try(k, t, filtStep{sl: sl, soleLife: true}, funded+1, gapUsed)
	}
	return false
}
