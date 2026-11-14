package dirac

// Cleave (CR 702.148a): "You may cast this spell by paying [cost] rather
// than paying its mana cost. If you do, change its text by removing all text
// found within square brackets." Cold, reached only when the table has a card
// with a printed cleave line (genPrep.anyCleave).
//
// gorge (engine/rules/cleave.go) offers the "cleaved" cast option at the
// cleave line's Cost$, with the offer's target census, the target ask and the
// resolution all reading the cleave line: a second spell ability whose
// parameters are already the bracket-free text. kgen lowers that line as a
// latent spell ability of its own (CardDef.CleaveAb is its index plus one,
// CardDef.CleaveCost its cost), and the kernel's cleaved cast simply puts that
// index on the stack entry in place of the printed ability's: the target
// stage, the resolution program and a stack copy (CR 707.10) all follow
// stackEntry.ab, so no second program or mode bit is needed. The entry is
// cleaved exactly when its ab is the card's cleave ability.
//
// gorge lists the cleaved cast as a legacy option, so only a floating pool
// that pays the cleave cost offers it (offerCleaveHand).
//
// The cast rides AkCast mode 0 with castCleaveTgt in the target byte, from
// the hand (the surge/overload shape); kgen refuses cleave beside any other
// alternative or additional cost.

// castCleaveTgt in an AkCast's target byte (mode 0, from the hand) is the
// cast for the card's cleave cost (offerCleaveHand).
const castCleaveTgt = 7

// FeatCastCleaveTgt is the target byte of a cleaved cast (canon/translate.go).
const FeatCastCleaveTgt = castCleaveTgt

// cleaved reports whether e is a spell cast for its cleave cost.
func (e *stackEntry) cleaved(k *prep) bool {
	if e.kind != seSpell || e.ab >= abCascade {
		return false
	}
	cd := &k.t.Cards[e.card]
	return cd.CleaveAb != 0 && e.ab == cd.CleaveAb-1
}

// offerCleaveHand offers hand card o's cleaved cast (gorge's "cleaved"
// option). The caller has checked the card's timing and the cast
// prohibition; it runs before the plain affordability gate, and the target
// census reads the cleave ability's own targets (the bracket-free text), so a
// cleaved cast is offered where the printed one has no legal target.
//
//go:noinline
func (s *State) offerCleaveHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	cd := &k.t.Cards[c]
	cc := s.adjustCost(k, p, c, o, cd.CleaveCost) // the modifiers apply to an alternative cost too (CR 601.2f)
	var fav avail
	cav := s.castGrantAvail(k, p, o, av, &fav)
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cc, cav) || k.sacMana && !s.kickWitness(k, p, c, o, &cc) {
		return
	}
	if !s.cleaveTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.beamPoolPays(k, p, o, cd.CleaveCost) {
		// gorge's planner prices no cleaved cast (it is no potential play
		// either): the option exists only when the floating pool pays it, as
		// a beam me up cast's (kw_beammeup.go beamPoolPays)
		return
	}
	d.add(MakeAction(AkCast, TCode(o), castCleaveTgt, 0))
}

// cleaveTargetsFeasible is spellTargetsFeasible for card c's cleave ability.
func (s *State) cleaveTargetsFeasible(k *prep, c CardID, p uint8) bool {
	a := &k.t.Cards[c].Abilities[k.t.Cards[c].CleaveAb-1]
	if k.gen != nil && s.g.probe.kind == 0 {
		// the spell's colours decide what it may target (as probeSpell)
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
		ok := s.abilityTargetsFeasible(k, a, p, NoSlot, 0xff)
		s.g.probe = probe{}
		return ok
	}
	return s.abilityTargetsFeasible(k, a, p, NoSlot, 0xff)
}
