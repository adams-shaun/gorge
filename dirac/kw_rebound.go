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
// a ReboundAb body, the entry's cast zone (stackEntry.prov, recorded because
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
	return !fizzled && k.gen != nil && k.gen.anyRebound && e.kind == seSpell &&
		k.t.Cards[e.card].ReboundAb != 0 && kop.Zone(e.prov&7) == kop.ZoneHand &&
		s.objs[e.obj].of&ofCopy == 0
}

// reboundRegister registers the your-next-upkeep delayed program of the
// rebounding spell entry e whose card has just been exiled. The registration
// is the card's new object (CR 400.7), so its sv is the ver the move gave it
// and the body's CastFree names it; a card a replacement sent elsewhere
// registers nothing.
//
//go:noinline
func (s *State) reboundRegister(k *prep, e *stackEntry) {
	if s.objs[e.obj].zone != kop.ZoneExile {
		return
	}
	if s.g.ndp >= maxDProg {
		s.capFull(StallDProg)
		return
	}
	d := dprog{card: e.card, ab: k.t.Cards[e.card].ReboundAb - 1, ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver,
		n: 1 | uint8(kop.DelayYourUpkeep)<<dpWhenShift}
	d.objs[0] = NoObj
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}
