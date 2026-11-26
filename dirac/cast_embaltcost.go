package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Effect-delivered AlternativeCost static (Marshland Bloodcaster's
// "Rather than pay the mana cost of the next spell you cast this turn, you
// may pay life equal to that spell's mana value"; gorge rules
// alternativeCosts' registry arm, engine/rules/statics_altcost.go, CR 118.9,
// 119.4). The activation's Effect resolves into an EOT emblem whose
// StaticAbilities$ body lowers as the kernel's one StatAlternativeCost
// static (kgen lower_effectaltcost.go). gorge's hand walk lists the
// alternative as its own AltCostIndex "cast" option whether or not the plain
// price is payable (legal_walk_hand.go's alternativeCosts loop), so the
// offer rides before the plain affordability gates beside offerMayPlayHandAlt
// (cast_mayzeroalt.go). The price is computed at the offer: the spell's mana
// value in life (payLifeOwed, paid over the free cast through
// bld().payLifeMV like a may-play permission's pay-life alternative,
// cast_mayplaypaylife.go), gated on payLifeOfferable (CR 119.4's
// no-offer-below-the-mana-value) beside the spell's own costs and the cost
// statics' increases, which the alternative still owes (CR 118.9d, 601.2f).
// The grant ends at end of turn (the emblem's UntilEOT) and on the
// controller's next completed cast (ForgetOnCast$ Card.YouCtrl as the
// emblem's Forget filter; kernel embFlashForget behind prepEmblemCasts'
// anyEmbForget), whether or not that cast used the alternative (gorge
// effectCastSweep). Cold: every site sits behind genPrep.anyEmbAltCost, and
// the read lives in embAlts' emblem scan (the embFlash shape,
// cast_selfflash.go).

// DebugEmbAltCostOff is the mutation switch (dirac/oracle
// TestEffectAltCostMutation): the emblem scan reads false, so no alternative
// is offered or marked, while gorge still lists it -- the lockstep must
// diverge.
var DebugEmbAltCostOff bool

// StallEmbAltShared: a second such emblem (a second activation or a second
// Bloodcaster) would put a second identical alternative on gorge's list
// (AltCostIndex 2), which the kernel's one embAltTgt target byte cannot
// name; so does the card's own AlternativeCost static, whose cast the kernel
// canonizes as the same AltCostIndex-1 cast (cast_altfree.go), and a free
// may-play admission whose cast shares the hand free-cast arm
// (cast_mayzeroalt.go). The game stalls loudly rather than list a cast the
// canon cannot tell apart.
const StallEmbAltShared = 20

// embAltTgt is the target byte of the Effect-delivered alternative's cast
// action: it tells the alternative from the plain hand cast (mode 0) and
// from a permission's free cast of the same card (castMayPlay|castFree with
// target 0), whose mode it shares; the apply side (castFlagsGenTgt) marks
// the build payLifeMV only for it. The canon maps it to gorge's
// alternativeCosts cast, mode "", AltCostIndex 1, beside the card's own
// alternative (castAltTgt) and the free may-play cast from the hand
// (translate.go's hand arm), which those sites' stalls keep apart.
const embAltTgt = 0xfc

func init() { stallNames[StallEmbAltShared] = "embaltshared" }

// embAlts counts the StatAlternativeCost statics player p's live emblems
// carry (the Effect-delivered alternative cost, the one lowered shape: every
// spell, no filter) and holds nothing back: a count above one cannot be
// named by the kernel's canon (StallEmbAltShared).
func (s *State) embAlts(k *prep, p uint8) int {
	if DebugEmbAltCostOff {
		return 0
	}
	n := 0
	for i := 0; i < int(s.g.nemb[p]); i++ {
		em := &k.t.Emblems[s.embIdx(p, i)-1]
		for j := range em.Abilities {
			a := &em.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
				n++
			}
		}
	}
	return n
}

// tableEmbAltCost reports whether some table emblem carries the
// StatAlternativeCost static (genPrep.anyEmbAltCost).
func tableEmbAltCost(t *kop.Table) bool {
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			a := &t.Emblems[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
				return true
			}
		}
	}
	return false
}

// offerEmbAltCostHand offers hand card o's cast through the Effect-delivered
// alternative cost (gorge legal_walk_hand.go's alternativeCosts loop: listed
// whether or not the plain price is payable, so it rides before the plain
// affordability gates like offerAltHand's own alternative). The alternative's
// action carries embAltTgt: the apply marks the build payLifeMV over the
// plain cast below (castFlagsGenTgt). A second such emblem, the card's own
// alternative cost or a free may-play admission stalls (StallEmbAltShared:
// two of gorge's casts would share one canon code).
//
//go:noinline
func (s *State) offerEmbAltCostHand(k *prep, d *Decision, p uint8, o Obj, c CardID, sorc bool, av *avail) {
	if k.gen == nil || !k.gen.anyEmbAltCost || k.cards[c].land {
		return
	}
	n := s.embAlts(k, p)
	if n == 0 {
		return
	}
	if n > 1 || k.cards[c].altCost {
		s.capFull(StallEmbAltShared)
		return
	}
	if rest, _ := s.mayPlayParts(k, p, o); rest.free {
		// a free may-play admission also offers its AltCostIndex-1 cast,
		// which the kernel canonizes as the hand arm's free cast (canon
		// translate.go); the two cannot share the code
		s.capFull(StallEmbAltShared)
		return
	}
	if !s.payLifeOfferable(k, p, o) {
		return // the life does not reach the mana value beside the spell's own (CR 119.4)
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	if s.freeOwes(k) {
		// the cost statics' increases are still owed over the replaced mana
		// cost (CR 118.9d, 601.2f): the offer stands only while the
		// floating pool pays them (the planner-surface rule the altcost
		// lockstep pinned, cast_altfree.go's free branch)
		if ac := s.adjustCost(k, p, c, o, kop.ManaCost{}); ac != (kop.ManaCost{}) && (!av.affords(&ac, pipMaskOf(&ac), 0) || !s.windowPoolPays(p, &ac, false)) {
			return
		}
	}
	act := MakeAction(AkCast, TCode(o), embAltTgt, castMayPlay|castFree)
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == act {
			return
		}
	}
	n0 := int(d.N)
	s.offerPlay(k, d, p, o, castMayPlay|castFree, sorc, false, false, false, false, av)
	plainAct := MakeAction(AkCast, TCode(o), 0, castMayPlay|castFree)
	for i := n0; i < int(d.N); i++ {
		if d.Options[i] == plainAct {
			d.Options[i] = act // the cast just offered is the alternative
		}
	}
}

// embAltCastMark marks the build of a castMayPlay|castFree hand cast taken
// through the Effect-delivered alternative cost (target embAltTgt; the
// offer's gates hold at the take, the same decision): the mana cost is
// replaced by the spell's mana value in life and no permission is spent
// (castFlagsGenTgt).
func (s *State) embAltCastMark(k *prep, p uint8, o Obj) bool {
	if k.gen == nil || !k.gen.anyEmbAltCost || s.embAlts(k, p) != 1 {
		return false
	}
	s.bld().payLifeMV = true
	return true
}
