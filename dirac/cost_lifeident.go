package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kop.Cost.LifeIdent: "Pay life equal to the number of colors in your
// commanders' color identity" (War Room; CR 903.4, 119.4), with the format
// gate that rides beside it, kop.Ability.CmdOnly ("Activate only in a game of
// Commander"; gorge activationGameTypesOK).
//
// gorge folds the count into a fixed life part wherever the cost is priced or
// paid (pay.FixLifeXCost over Host.CommanderIdentityColourCount), so the part
// is a fixed PayLife<N> whose N is the payer's own:
//
//   - the offer is withheld while the payer's life is below it (canActivate,
//     through costTokenPartsPayable; CR 119.4);
//   - an activation the mana window's own life payments left short reverses
//     (cast.go, the fixed PayLife<N> recheck; CR 733.1);
//   - the payment is one life loss beside the other cost parts
//     (payAbilityCosts), and none for a count of 0: a colourless commander,
//     or a seat with no commander (CR 903.4f).
//
// The count is the size of the union of the colour identities of the payer's
// commanders, a partner's too, so a colour both partners share is paid once
// (gorge pay.CommanderIdentityColours). The payer is the activating player,
// whoever owns the source.

// DebugLifeIdentFree is the mutation switch: the part owes no life.
// TestLifeIdentLockstepMutation requires it to diverge.
var DebugLifeIdentFree bool

// lifeIdentOwed is the life a LifeIdent cost charges seat p.
//
//go:noinline
func (s *State) lifeIdentOwed(k *prep, p uint8) int {
	if DebugLifeIdentFree {
		return 0
	}
	var m kop.ColorMask
	p = SeatIx(p)
	for j := 0; j < 2; j++ {
		if o := s.cmdObj(p, j); o != NoObj {
			m |= k.t.Cards[s.objs[o].card].Identity
		}
	}
	return bits.OnesCount8(uint8(m & kop.ColAny))
}

// payLifeIdent pays seat p's LifeIdent part.
func (s *State) payLifeIdent(k *prep, p uint8) {
	n := s.lifeIdentOwed(k, p)
	if st := DebugLifeIdentStats; st != nil {
		st.Pays++
		st.ByCount[min(n, len(st.ByCount)-1)]++
	}
	s.loseLife(k, p, n)
}

// LifeIdentStats counts the LifeIdent payments (test hook).
type LifeIdentStats struct {
	Pays    int    // payments made
	ByCount [6]int // payments by the life charged: 0 to 5 colours
}

// DebugLifeIdentStats is the test hook; nil in normal play.
var DebugLifeIdentStats *LifeIdentStats

// actTokenPartsPayable is canActivate's gate over the cost parts that need no
// pick (costTokenPartsPayable) for ability a, behind the format gate: a
// Commander-only ability is offered in no other game.
func (s *State) actTokenPartsPayable(k *prep, p uint8, src Obj, a *kop.Ability) bool {
	return (!a.CmdOnly || s.cfg&cfgCommander != 0) && s.costTokenPartsPayable(k, p, src, &a.Cost)
}

// buildLifeShort reports whether the activation being built owes its
// controller p more life than p has left: a fixed PayLife<N> (the PayLife<X>
// form owes 0 here and is bounded by its X ask) or a LifeIdent part. The
// offer priced both, but the mana window's own life payments come after it.
func (s *State) buildLifeShort(k *prep, p uint8) bool {
	pc := s.buildPickCost(k)
	if pc == nil {
		return false
	}
	if pc.Flags&kop.CostPayLife != 0 && int(s.pl[SeatIx(p)].life) < lifeCostOwed(pc) {
		return true
	}
	return !s.lifeIdentPayable(k, p, pc)
}

// lifeIdentPayable reports whether seat p can pay c's LifeIdent part: a
// player pays life only out of a total at least that large (CR 119.4), and
// zero is always payable (CR 119.4b).
func (s *State) lifeIdentPayable(k *prep, p uint8, c *kop.Cost) bool {
	return !c.LifeIdent || int(s.pl[SeatIx(p)].life) >= s.lifeIdentOwed(k, p)
}
