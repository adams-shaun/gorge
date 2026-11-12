package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Retrace (CR 702.81a), cold: reached only when the table has a card with
// printed Retrace (prep.retrace).
//
// A Retrace card in its owner's graveyard may be cast by paying its printed
// mana cost PLUS the additional cost of discarding a land card (gorge rules
// cast_begin.go's castModeRetrace arm through retraceExtra; legal_walk_alt.go's
// graveyard walk, Option Kind "cast" Mode "retrace"). Unlike the
// alternative-cost keyword family Retrace is no cost substitution: the
// printed base (castOfferBase's composition) stays and only the additional
// discard folds on, so the cast's mana price is the card's printed Cost and
// the discard is its pick cost (CardDef.RetraceCost, CostDiscardFiltered).
//
// The cast is an ordinary one from the graveyard: the spell rests in its
// owner's graveyard on resolution (no sfFlashback exile), and gorge's
// wasCastFromGraveyard reads true through the object's latest cast zone
// (cast_fromgrave.go), not through a flag -- gorge stamps no cast flag for
// it (nomanacost.go lists "retrace" among the printed-mana-cost modes).
// The kernel action is AkCast with retraceTgt in the target byte and mode
// bit 2; beginCast marks the entry trigPlRetrace, which buildPickCost reads
// for the RetraceCost. A copy keeps no mark (mintSpellCopy, CR 707.10).
//
// A card with a stamped or live FLASHBACK grant beside its printed Retrace
// casts both ways (gorge lists both options): the flashback instance is the
// plain mode-2 cast (target byte 0, the granted instance's cost, sfFlashback
// and the resolution exile), the retrace one the marked action. kgen refuses
// the printed-keyword collisions (Flashback, Harmonize, Mayhem, BeamMeUp),
// so the target byte alone separates the two instances of one card.

// retraceTgt is the retrace cast's target-byte mark, shared with canon
// (translate.go) and the oracle (sbt.go), which canonicalise it as gorge's
// "retrace" option (CastModeOther, like "harmonize"). 1 is free for a
// mode-2 cast: the plain flashback and the beam me up permission indexes
// never sit on a Retrace card (kgen refuses those keywords beside it).
const retraceTgt uint8 = 1

// FeatRetraceTgt is the target byte of a retrace cast (canon/translate.go
// and the oracle read the canonical mark through it).
const FeatRetraceTgt = retraceTgt

// retraceEntry reports whether the pending spell e is a retrace cast.
func retraceEntry(cd *kop.CardDef, e *stackEntry) bool {
	return cd.Retrace && e.trigPl == trigPlRetrace
}

// DebugRetraceFree is the mutation switch (dirac/oracle TestRetraceMutation):
// the cast's pick cost asks nothing, so the cast resolves without charging
// the discard and the lockstep must diverge on the land never discarded.
var DebugRetraceFree bool

// retraceCost is the cast's pick cost: the additional discard-a-land cost
// (gorge retraceExtra through the one definition the offer gate prices).
func (s *State) retraceCost(k *prep, c CardID) *kop.Cost {
	rc := &k.t.Cards[c].RetraceCost
	if DebugRetraceFree {
		return nil
	}
	if rc.Flags == 0 {
		return nil // never built (kgen refuses a Retrace card without its cost)
	}
	return rc
}

// offerRetraceCasts offers the graveyard casts of p's Retrace cards (gorge
// legal_walk_alt.go's retrace walk: spellTimingOK, castRestricted, castTargets,
// and the discard payable gate -- an option whose discard cannot be paid must
// never be offered). The mana is the printed cost priced with the walk's own
// reductions (gorge offerCastable's offerCostFor); the cast's pick cost is
// gated separately by pickCostPayable, and the floating-pool rule prices the
// offer like the mayhem cast's (gorge's payment planner plans no retrace
// mode).
func (s *State) offerRetraceCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	have := false
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		cd := &k.t.Cards[c]
		if !cd.Retrace {
			continue
		}
		if !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if k.gen != nil && k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !have {
			s.availFor(k, p, av)
			have = true
		}
		fc := s.adjustCost(k, p, c, o, cd.Cost)
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) || !s.spellTargetsFeasibleFrom(k, c, p, o) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		if rc := &k.t.Cards[c].RetraceCost; rc.Flags == 0 || !s.pickCostPayable(k, p, rc, o) {
			continue // the additional discard-a-land cost (CR 601.2h, 702.81a)
		}
		if s.castHeldOut(o) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.FeatPoolPays(p, o, 2|8) {
			// gorge's payment planner plans no retrace mode (payment_plan.go
			// plans plain, may-play and flashback casts only): the cast is
			// offered only when the floating pool pays the printed base (the
			// mayhem cast's rule)
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), retraceTgt, 2))
	}
}

// FeatRetraceCard reports whether o's card is a Retrace card (the oracle
// canonicalises its retrace cast as gorge's "retrace" option).
func (s *State) FeatRetraceCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Retrace
}
