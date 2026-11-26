package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Rebound (CR 702.88a): a spell cast from its controller's hand is exiled as
// it resolves instead of going to the graveyard and, at the beginning of its
// controller's next upkeep, may be cast from exile without paying its mana
// cost. gorge stamps FlagRebound at pay time for a hand-origin cast of a face
// carrying the keyword (cast_commit.go), sends the RESOLVED spell to exile
// (a countered or fizzled one goes to the graveyard: spellRestZone against
// spellFizzleZone) and, only if the card reached exile, registers the
// __kwReboundCast delayed trigger (resolution_chain.go registerRebound).
//
// The kernel reads the same three facts off the resolving entry: the card has
// a ReboundAb body (or a live StatStackKw Rebound grant names the carrier's), the entry's cast zone (stackEntry.prov, recorded because
// genPrep.anyCastProv is set for any such table) is the hand, and the spell
// is not a copy (a copy is put on the stack, never cast: CR 707.10). The
// recast from exile carries prov = exile, so it does not rebound again.
//
// reboundRests is read by every exit of a resolved spell (finishResolve, the
// held-spell exit, parkSpellExit) so none can miss the exile; reboundRegister
// runs after the move, only when the card is in exile.

// reboundRests reports whether resolved (not fizzled) spell entry e is
// exiled by its Rebound.
func (s *State) reboundRests(k *prep, e *stackEntry, fizzled bool) bool {
	_, _, ok := s.reboundPlan(k, e, fizzled)
	return ok
}

// reboundPlan is reboundRests that also names the recast body the upkeep
// program runs: the spell card's own ReboundAb for a printed Rebound, else
// the GrantBody of the carrier of a live StatStackKw Rebound grant (Cast
// Through Time; gorge spellReboundGranted reads the derived keyword as the
// spell resolves, a printed face being stamped at cast instead). It must run
// while the spell is still on the stack: the grant's filter reads it there.
func (s *State) reboundPlan(k *prep, e *stackEntry, fizzled bool) (card CardID, ab uint8, ok bool) {
	if fizzled || k.gen == nil || !k.gen.anyRebound || e.kind != seSpell ||
		kop.Zone(e.prov&7) != kop.ZoneHand || s.objs[e.obj].of&ofCopy != 0 {
		return 0, 0, false
	}
	if rb := k.t.Cards[e.card].ReboundAb; rb != 0 {
		return e.card, rb - 1, true
	}
	if !k.gen.anyStackKw {
		return 0, 0, false
	}
	n, _, src := s.stackKwScan(k, kop.StackKwRebound, e.card, e.obj, e.ctrl, true)
	if n == 0 || src.sl == NoSlot || src.emb != 0 || src.gr {
		return 0, 0, false
	}
	return s.cardAt(src.sl), src.a.Static.GrantBody - 1, true
}

// reboundRegister registers the your-next-upkeep delayed program of the
// rebounding spell entry e whose card has just been exiled. The registration
// is the card's new object (CR 400.7), so its sv is the ver the move gave it
// and the body's CastFree names it; a card a replacement sent elsewhere
// registers nothing.
//
//go:noinline
func (s *State) reboundRegister(k *prep, e *stackEntry, card CardID, ab uint8) {
	if s.objs[e.obj].zone != kop.ZoneExile {
		return
	}
	if s.g.ndp >= maxDProg {
		s.capFull(StallDProg)
		return
	}
	d := dprog{card: card, ab: ab, ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver,
		n: 1 | uint8(kop.DelayYourUpkeep)<<dpWhenShift}
	d.objs[0] = NoObj
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}
