package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Surge (CR 702.117a): "You may cast this spell for its surge cost if you or
// a teammate has cast another spell this turn." Cold, reached only when the
// table has a card with printed surge (genPrep.anySurge).
//
// gorge's hand walk (legal_walk_hand.go) offers the "surged" cast option
// when the face has the keyword, pay.SpellsCastThisTurn(p) > 0 (the caster's
// PutOnStack events since the turn began, reversals undone) and
// offerCastable prices the surge cost alone, under the plain cast's timing
// gate but before (and independent of) its affordability gate; beginCast's
// castModeSurged replaces the printed cost with the surge cost (an
// alternative cost, CR 118.9) and modeFlags stamps FlagSurged. The kernel
// reads the caster's spells-cast count off the history bank
// (hist[p][histSpells], written by histCast as each cast is paid; anySurge
// turns anyHist on). Two players have no teammates.
//
// gorge's payment planner plans no surged cast; its potential planner prices
// the composed surge cost (potential_plan.go potentialModeBaseCostSurged),
// the kicked cast's shape: beginCast taps that witness at priority and the
// cast pays from the floating pool, as the kicked cast does (cast.go).
//
// The cast rides AkCast mode 0 with castSurgeTgt in the target byte;
// legal.go sets genState.impCast and beginCast marks the build entry with
// modeImpended, which reads as "surged" on a HasSurge card (kgen refuses
// surge beside every other keyword that claims the bit, and on a modal
// spell, whose chosen modes share stackEntry.mode). A stack copy clears the
// bit (copy_gen.go). The FlagSurged readers (Card.Self+surged, Condition$
// Surge) stay refused in kgen.

// castSurgeTgt in an AkCast's target byte (mode 0, from the hand) is the cast
// for the card's surge cost (offerSurgeHand).
const castSurgeTgt = 6

// FeatCastSurgeTgt is the target byte of a surged cast (canon/translate.go).
const FeatCastSurgeTgt = castSurgeTgt

// DebugSurgeAnyTurn drops the "another spell this turn" gate
// (TestMBCSurgeGateMutation: the lockstep must catch it).
var DebugSurgeAnyTurn bool

// surged reports whether e is a spell cast for its surge cost.
func (e *stackEntry) surged(k *prep) bool {
	return e.kind == seSpell && k.t.Cards[e.card].HasSurge && e.mode&(modeImpended|seFaceDown) == modeImpended
}

// offerSurgeHand offers hand card o's surged cast (gorge's "surged" option).
// The caller has checked the card's timing and the cast prohibition; it runs
// before the plain affordability gate.
//
//go:noinline
func (s *State) offerSurgeHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if s.g.hist[SeatIx(p)][histSpells] == 0 && !DebugSurgeAnyTurn {
		return // no other spell cast this turn (CR 702.117a)
	}
	sc := s.adjustCost(k, p, c, o, k.t.Cards[c].SurgeCost) // the modifiers apply to an alternative cost too (CR 601.2f)
	var fav avail
	cav := s.castGrantAvail(k, p, o, av, &fav)
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &sc, cav) || k.sacMana && !s.kickWitness(k, p, c, o, &sc) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	d.add(MakeAction(AkCast, TCode(o), castSurgeTgt, 0))
}

// surgeCostOf is the surged cast's printed alternative cost.
func surgeCostOf(cd *kop.CardDef) kop.ManaCost { return cd.SurgeCost }
