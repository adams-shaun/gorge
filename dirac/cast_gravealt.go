package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// offerGraveyardAltCasts offers the graveyard casts of p's cards whose own
// StatAlternativeCost static names the graveyard (kop.Static.Zone
// ZoneGraveyard, prepAltFree; gorge legal_walk_alt.go's
// graveyardAltCostCasts: Worldheart Phoenix, Raffine's Guidance, Squee, Dubious
// Monarch, Scourge of Nel Toth, CR 601.2, 118.9). The hand walk never offers
// them (offerAltFreeHand declines a graveyard-scoped static), and an
// EffectZone$ All one is not this permission (Static.Zone stays ZoneNone) --
// gorge's graveyardSelf bit, statics_altcost.go. The action is the hand
// alternative's (mode 0, target castAltTgt): legal.go sets genState.altCast
// for it, beginCast prices and pays the static's cost (buildAltCost) and the
// push derives the origin from the object's zone, so the card moves from the
// graveyard (unpushSpell returns it there). The oracle canonises both origin
// hands to gorge's Mode "" AltCostIndex 1 cast (canon translate.go).
//
// The gates are offerAltFreeHand's at the graveyard: the static's condition,
// the spell's timing (an instant any time, else sorcery speed, like
// offerEscapeCasts), the cast prohibition, the targets from a graveyard
// origin, the pick parts' census and the mana. The mana alternative is
// offered only when the floating pool pays it (planner-surface rule,
// offerAltFreeHand's comment; gorge's walk gates on the pool too). The spell's
// own additional costs are paid beside the alternative (CR 118.9d), the same
// gate the hand arm prices (spellCostsFeasible).
//
//go:noinline
func (s *State) offerGraveyardAltCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	have := false
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		if k.gen.altFree == nil || k.gen.altFree[c] == 0 {
			continue
		}
		a := &k.t.Cards[c].Abilities[k.gen.altFree[c]-1]
		if a.Static.Zone != kop.ZoneGraveyard {
			continue
		}
		if a.Static.Cond2 != 0 && !s.condEval(k, a.Static.Cond2, evalCtx{src: o, srcSl: NoSlot, ctrl: p}) {
			continue
		}
		if !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue // the card's normal timing (CR 601.2)
		}
		if k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !have {
			s.availFor(k, p, av)
			have = true
		}
		if !s.spellTargetsFeasibleFrom(k, c, p, o) {
			continue // the cast asks its targets from a graveyard origin
		}
		if a.Cost.Mana == (kop.ManaCost{}) {
			continue // a free alternative is the hand walk's shape; no corpus graveyard static lowers one
		}
		ac := s.adjustCost(k, p, c, o, a.Cost.Mana)
		if altPickCost(&a.Cost) && !s.pickCostPayable(k, p, &a.Cost, o) {
			continue // the sacrifice (Scourge) or graveyard-exile (Squee) picks are not there
		}
		if !av.affords(&ac, pipMaskOf(&ac), 0) || !s.windowPoolPays(p, &ac, false) {
			continue
		}
		if s.castHeldOut(o) {
			continue
		}
		if !s.spellCostsFeasible(k, c, p, o) {
			continue // the spell's own additional costs (CR 118.9d)
		}
		d.add(MakeAction(AkCast, TCode(o), castAltTgt, 0))
	}
}
