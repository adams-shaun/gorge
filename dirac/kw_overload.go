package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Overload (CR 702.96a): "You may cast this spell for its overload cost. If
// you do, change its text by replacing every instance of the word 'target'
// with the word 'each.'"
//
// The hand offers the overloaded cast at CardDef.OverloadCost (gorge's
// "overloaded" option, altcast_modes.go altOverload: a potential play with no
// target check, offered like warp/impending/evoke before the plain
// affordability gate). It rides AkCast mode 0 with castOverloadTgt in the
// target byte; the cast's target stage asks nothing (the spell has no
// targets), and the resolution runs CardDef.OverloadProg, kgen's each-variant
// of the printed program, instead of the printed spell ability. A stack copy
// clears the mode bit (copy_gen.go), so a copy is not overloaded.
//
// Provenance rides stackEntry.mode's impending bit: a card has overload or
// impending/dash, never both (kgen refuses the combination), so the bit reads
// as overloaded only on a HasOverload card. The bit is also the one
// copy_gen.go clears for impending/dash/dash casts.

// castOverloadTgt in an AkCast's target byte (mode 0, from the hand) is the
// cast for the card's overload cost (offerOverloadHand).
const castOverloadTgt = 5

// overloaded reports whether e is a spell cast for its overload cost.
func (e *stackEntry) overloaded(k *prep) bool {
	return e.kind == seSpell && k.t.Cards[e.card].HasOverload && e.mode&(modeImpended|seFaceDown) == modeImpended
}

// offerOverloadHand offers hand card o's overloaded cast (gorge's
// "overloaded" option). The caller has checked the card's timing and the
// cast prohibition; it runs before the plain affordability gate, and it does
// not require a legal target: overload's "each" has no target to announce
// (CR 702.96b), so the cast is legal with no matching permanent at all.
func (s *State) offerOverloadHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	cd := &k.t.Cards[c]
	oc := s.adjustCost(k, p, c, o, cd.OverloadCost) // the reduction applies to an alternative cost too (CR 601.2f)
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits; a hand-barred grant puts no cast on
		// the walk grantCap reads (as offerImpendingHand)
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	if !av.affords(&oc, pipMaskOf(&oc), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &oc, av)) {
		return
	}
	if !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &oc, av) {
		return
	}
	if s.castHeldOut(o) {
		return
	}
	d.add(MakeAction(AkCast, uint8(o), castOverloadTgt, 0))
}

// overloadProg is the program the resolving entry runs: the overloaded
// cast's each-variant, else the printed spell ability's program.
func (s *State) overloadProg(k *prep) []kop.Instr {
	e := &s.res.e
	if e.overloaded(k) && len(k.t.Cards[e.card].OverloadProg) != 0 {
		return k.t.Cards[e.card].OverloadProg
	}
	return nil
}
