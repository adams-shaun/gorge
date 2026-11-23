package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A deferred Produced$ ColorIdentity source (Path of Ancestry's "{T}: Add one
// mana of any color in your commander's color identity. When that mana is
// spent to cast a creature spell that shares a creature type with your
// commander, scry 1", kop.Ability.ManaDeferred beside ManaIdentityPotential).
// gorge's planner defers it like any TriggersWhenSpent$ source
// (mana_deferred.go), so its census never lists it, and its potential fold
// prices it at the controller's commander identity (CR 903.4; legal.go
// identityPotentialAdd). The kernel treats it as the deferred gap it is, with
// the one difference that its colour is an ask over the identity's colours: one
// mana, the pick named by the play's announced colours first (gorge's
// scriptOrderAnswers), an identity of one colour asking nothing and an empty
// one (CR 903.4f) making no mana, so no gap at all.

// deferIdentAb reports whether a is a deferred ColorIdentity ability.
func deferIdentAb(a *kop.Ability) bool { return a.ManaDeferred && a.ManaIdentityPotential }

// deferIdent reports whether card c's deferred ability is a ColorIdentity one.
func (k *prep) deferIdent(c CardID) bool {
	a := k.deferAb(c)
	return a != nil && deferIdentAb(a)
}

// deferMask is the colours the deferred source at sl may make now: its one
// fixed colour, or the controller's commander identity.
func (s *State) deferMask(k *prep, sl Slot) uint8 {
	c := s.cardAt(sl)
	if k.deferIdent(c) {
		return uint8(s.identityColours(k, s.perms[sl].ctrl))
	}
	return 1 << k.deferCol(c)
}

// deferNoMana reports whether the deferred source at sl makes no mana: an
// identity source whose controller's commanders have no colour (CR 903.4f).
func (s *State) deferNoMana(k *prep, sl Slot) bool {
	return k.deferIdent(s.cardAt(sl)) && s.identityColours(k, s.perms[sl].ctrl) == 0
}

// deferAnswers is the colour ask's answer count at sl: the identity's colours,
// one when the colour is fixed or the identity has a single colour.
func (s *State) deferAnswers(k *prep, sl Slot) uint8 {
	return uint8(max(bits.OnesCount8(s.deferMask(k, sl)), 1))
}

// deferOrder is the colour answers of the deferred source at sl in the search's
// order: the play's announced colours first, the rest in engine order (W U B R G).
func (s *State) deferOrder(k *prep, sl Slot, need *[kop.NumManaKinds]bool, out *[6]uint8) []uint8 {
	m := s.deferMask(k, sl)
	n := 0
	for pass := 0; pass < 2; pass++ {
		for c := 0; c < kop.NumManaKinds; c++ {
			if m&(1<<c) != 0 && need[c] == (pass == 0) {
				out[n] = uint8(c)
				n++
			}
		}
	}
	return out[:n]
}

// deferChainTap taps the deferred source at sl for the script search's answer c
// of play a: the c'th colour of the ask in deferOrder.
func (s *State) deferChainTap(k *prep, p uint8, a Action, sl Slot, c int) {
	need := s.playNeed(k, a)
	var out [6]uint8
	cols := s.deferOrder(k, sl, &need, &out)
	col := -1
	if len(cols) != 0 {
		col = int(cols[min(c, len(cols)-1)])
	}
	s.tapGapDeferred(k, p, sl, col)
}

// tapGapDeferred taps the deferred source at sl for its mana of colour col
// (-1: the lowest colour of deferMask) and floats it as the source's.
func (s *State) tapGapDeferred(k *prep, p uint8, sl Slot, col int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if a := k.deferAb(s.cardAt(sl)); a != nil && a.Cost.Flags&kop.CostPayLife != 0 {
		s.payLifeCost(k, p, &a.Cost, 0) // Boseiju's "Pay 2 life" (mana_deferred.go deferLifeSurvives)
	}
	s.floatDeferredAs(k, p, sl, col)
}

// windowDeferIdentTap activates the deferred identity source sl in a payment
// window: the tap, then the one mana in the colour the cost still owes (windowOwedCost),
// else the lowest colour the clamp keeps, else the lowest of the identity. A
// single colour asks nothing.
func (s *State) windowDeferIdentTap(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool) {
	m := s.deferMask(k, sl)
	col := bits.TrailingZeros8(m)
	if m == 0 {
		col = -1
	} else if m&(m-1) != 0 {
		if !noOwed {
			var owed kop.ManaCost
			s.windowOwedCost(k, p, cost, &owed)
			for c := 0; c < kop.NumManaKinds; c++ {
				if m&(1<<c) != 0 && owed.Pips[c] > 0 {
					col = c
					break
				}
			}
		} else if clamp {
			if want, decided := s.windowUseful(k, p, useful); decided {
				if wm := m & want; wm != 0 {
					col = bits.TrailingZeros8(wm)
				}
			}
		}
	}
	s.tapGapDeferred(k, p, sl, col)
}

// deferIdentOther branches the filter search's deferred identity gap sl on its
// colour ask (gorge's scriptOrderAnswers: the play's announced colours first),
// one ask node when the identity holds several colours.
func (fs *filtSearch) deferIdentOther(k *prep, t *State, sl Slot, try func(otherStep) bool) bool {
	var out [6]uint8
	cols := t.deferOrder(k, sl, &fs.need, &out)
	if len(cols) == 0 {
		return false
	}
	if len(cols) > 1 && !fs.tick() {
		return false
	}
	for _, c := range cols {
		if try(otherStep{kind: otherDeferred, col: int8(c)}) {
			return true
		}
	}
	return false
}
