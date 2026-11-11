package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Buyback (CR 702.27a, Time Spiral), copied from gorge (rules
// cast_altcost.go buybackCost, legal_walk_hand.go's "(buyback)" offer,
// cast_begin.go castModeBuyback, stack.go spellRestZone):
//
//   - the buyback cost is an optional ADDITIONAL cost offered as its own
//     cast option beside the plain cast (gorge Mode "buyback", the Offspring
//     shape: base + buyback, never an ask). The kernel's kicked cast is that
//     option: CardDef.Buyback cards carry the buyback cost in CardDef.Kicker
//     (no KwKicker; a one-part Sac<1/..> cost is zero mana plus KickerCost,
//     kw_kicker.go) and the paid flag is sfKicked on the entry, so
//     stackEntry and State grow by nothing. kgen refuses a card with a
//     Kicker, an Offspring or another cast route beside it.
//   - the paid flag only matters as the spell leaves the stack: gorge's
//     spellRestZone puts a RESOLVED paid spell into its owner's hand instead
//     of the graveyard. A countered or fizzled spell (spellFizzleZone,
//     counterAt) goes to the graveyard as ever, a copy ceases to exist, and
//     the exile arms (flashback family, Paradigm, Adventure, Omen) win.
//     buybackRest is that one arm, shared by finishResolve and the held
//     spell's exit (trig_xpaid.go spellHoldLeaves).
//   - gorge's StackView.Kicks does not count it (FlagBuyback, not
//     FlagKicked): FeatStackKicked.

// DebugBuybackNeverReturns (mutation test only): a paid Buyback spell goes
// to the graveyard like an unpaid one.
var DebugBuybackNeverReturns bool

// buybackRest is the zone a spell resolving from e rests in: z (the zone
// the other arms chose) unless z is the graveyard and the spell's Buyback
// cost was paid, which returns it to its owner's hand.
func buybackRest(k *prep, e *stackEntry, z kop.Zone, fizzled bool) kop.Zone {
	if z != kop.ZoneGraveyard || fizzled || e.flags&sfKicked == 0 || DebugBuybackNeverReturns {
		return z
	}
	if int(e.card) < len(k.t.Cards) && k.t.Cards[e.card].Buyback {
		return kop.ZoneHand
	}
	return z
}
