package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// effectAltCostStatic lowers an Effect's StaticAbilities$ Mode$
// AlternativeCost body (Marshland Bloodcaster's SVar:ReduceCost, "Rather
// than pay the mana cost of the next spell you cast this turn, you may pay
// life equal to that spell's mana value"; gorge rules alternativeCosts'
// registry arm, engine/rules/statics_altcost.go, priced through altCostParse's
// ConvertedManaCost substitution, CR 118.9, 119.4). It is the one shape the
// two engines model: ValidSA$ Spell (any spell, alternativeCostScopeOK's
// bare-Spell arm), ValidPlayer$ You (the emblem's player, the same "You"
// embFlash's Caster$ You reads) and Cost$ exactly PayLife<ConvertedManaCost>
// -- the price is the cast spell's mana value, which exists only at the cast
// offer, so the lowered static carries no numeric cost: the kernel's emblem
// reader (cast_embaltcost.go) prices it payLifeOwed's mana value through
// bld().payLifeMV and gates the offer on payLifeOfferable (CR 119.4's
// no-offer-below-the-mana-value). A ValidCard$, an EffectZone$ or any other
// rider the registry arm reads is a shape the kernel's offer walk does not,
// so it refuses; the corpus card carries none.
//
// The lifetime is the Effect's own: an EOT emblem (Borne Upon a Wind's), and
// ForgetOnCast$ Card.YouCtrl becomes the emblem's Forget filter, ending the
// grant on the controller's next completed cast whether or not that cast
// used the alternative (gorge effectCastSweep; kernel embFlashForget's
// em.Forget arm, which prepEmblemCasts' anyEmbForget opens for it).
func (lc *lowerCtx) effectAltCostStatic(s cards.Static) (lStatic, error) {
	sp := newParams(s.Params)
	if sp.str("ValidSA") != "Spell" || sp.str("ValidPlayer") != "You" {
		return lStatic{}, refuse("Effect AlternativeCost shape")
	}
	if c := sp.str("Cost"); c != "PayLife<ConvertedManaCost>" {
		return lStatic{}, refuse("Effect AlternativeCost Cost$ %q", c)
	}
	sp.consume("Mode", "ValidSA", "ValidPlayer", "Cost", "Description")
	if left := sp.unread(); len(left) > 0 {
		return lStatic{}, refuse("Effect AlternativeCost unread %v", left)
	}
	// Kind AlternativeCost (build_ab.go's arm) with no cost and no filter:
	// the one AlternativeCost static an emblem carries, read by
	// cast_embaltcost.go's emblem scans alone.
	return lStatic{Kind: "AlternativeCost"}, nil
}
