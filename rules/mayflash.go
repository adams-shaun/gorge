package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	// The coverage census: kw:MayFlashCost is implemented as a casting option
	// read directly off the K: line (the Flash/Flashback/Bestow/Mutate family
	// -- see this file's doc), so it is registered here in its own file
	// exactly as bestow.go and mutate.go register theirs. Proof:
	// TestTegwyllsScouringMayflashCastTapsThreeFlyers plus the
	// plain-cast/withheld negatives in rules/mayflash_test.go.
	effects.RegisterNonAPI("kw:MayFlashCost")
}

// MayFlashCost (Forge's K:MayFlashCost, the "as though it had flash" casting
// option behind CR 702.8) — a card printed with the keyword may be cast at
// instant timing if its controller pays an ADDITIONAL cost:
//
//	K:MayFlashCost:2
//	  "You may cast Breaking Wave as though it had flash if you pay {2} more
//	   to cast it." (Rout, Ghitu Fire, Saproling Symbiosis, ...)
//	K:MayFlashCost:tapXType<3/Creature.withFlying/creatures with flying>
//	  Tegwyll's Scouring: "... if you tap three creatures with flying."
//	K:MayFlashCost:Behold<1/Dragon>
//	  Molten Exhale: "... if you behold a Dragon."
//
// The colon parameter is that extra cost, NOT a replacement for the printed
// mana cost: the oracle wording is "pay {2} MORE to cast it", so the charge
// is base.Plus(extra). The keyword is a CASTING OPTION, so — exactly like
// Flash, Flashback, Delve and Miracle — it is read directly by rules rather
// than expanded onto the face (the family cards/keywords.go's expandKeywords
// doc names). The pieces:
//
//   - mayflashExtraCost resolves the printed extra cost, the
//     replicate/bestow fail-closed convention: a token ParseCost cannot model,
//     or an {X} whose announcement is not proved end to end, withholds the
//     offer rather than charging a degraded generic.
//   - mayflashTimingOK is the timing gate the hand walk relaxes to. It still
//     honours activationPhasesOK (a spell whose own SA carries a phase
//     restriction stays confined) but skips the instant/sorcery-speed half of
//     spellTimingOK, because paying the extra IS the flash permission.
//   - the offer (rules/legal.go's hand walk) adds the mayflash cast option
//     when the ordinary timing gate fails, and beginCast folds the extra into
//     the charge.

// mayflashExtraCost resolves the printed MayFlashCost additional cost. An
// absent keyword, an empty parameter (K:MayFlashCost: with nothing after the
// colon — no corpus shape), a token ParseCost reported as Unknown, and any
// {X} all withhold: each would otherwise charge a cost that is not the card's.
func mayflashExtraCost(f *cards.Face) (Cost, bool) {
	if f == nil {
		return Cost{}, false
	}
	s, ok := f.KeywordParam("MayFlashCost")
	if !ok || s == "" {
		return Cost{}, false
	}
	c := ParseCost(s)
	if len(c.Unknown) > 0 || c.X > 0 {
		return Cost{}, false
	}
	return c, true
}

// mayflashTimingOK reports whether f may be cast at instant timing through its
// MayFlashCost permission: the keyword must be present and the face's own
// activation-phase restrictions (ActivationPhases$, PlayerTurn$, ...) must
// hold. The instant/sorcery-speed check spellTimingOK makes is deliberately
// omitted — paying the extra is what grants the flash window.
func (e *Engine) mayflashTimingOK(p state.PlayerID, f *cards.Face) bool {
	if f == nil {
		return false
	}
	if _, ok := f.KeywordParam("MayFlashCost"); !ok {
		return false
	}
	return e.activationPhasesOK(p, f.SpellAbility())
}
