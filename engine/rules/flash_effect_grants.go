package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Effect-delivered spell-flash grants ("you may cast spells this turn as
// though they had flash": Borne Upon a Wind, Quicken, Teferi, Time Raveler's
// +1, Alchemist's Refuge, Emergence Zone, Winding Canyons). effEffect
// registers the body in the continuous registry as a CastWithFlash
// restriction when effects.SpellFlashParamsReadable admits it, and the cast
// timing reads it here beside the printed S: statics castWithFlashTargets
// walks, so the two delivery routes answer one question. Caster$ You is the
// effect's controller (the registration's Controller).

// spellFlashRegistration reports whether ce is a live-form spell-flash grant.
func spellFlashRegistration(ce *ContinuousEffect) bool {
	return ce.Restriction == "CastWithFlash" && effects.SpellFlashParamsReadable(ce.RestrictParams)
}

// effectFlashLive reports whether any active effect delivers a spell-flash
// grant: the emptiness half of castWithFlash's fast path and the walk's board
// bit, beside len(activeStatics("CastWithFlash")).
func (e *Engine) effectFlashLive() bool {
	ces := e.active()
	for i := range ces {
		if spellFlashRegistration(&ces[i]) {
			return true
		}
	}
	return false
}

// effectFlashGrants reports whether an Effect-delivered spell-flash grant of
// player p's lets p cast id as though it had flash: ValidSA$ Spell against
// the potential targets, ValidCard$ against the card, the registration's
// remembered set bound like every other Effect-delivered spec.
func (e *Engine) effectFlashGrants(p state.PlayerID, id state.ObjID, f *cards.Face, targets []state.Target, teamworkOffer bool) bool {
	ces := e.active()
	for i := range ces {
		ce := &ces[i]
		if !spellFlashRegistration(ce) || ce.Controller != p {
			continue
		}
		if !e.spellMatchesValidSA(f, ce.RestrictParam(cards.PKValidSA), id, ce.Source, p, targets, teamworkOffer) {
			continue
		}
		sc := e.specCtx(ce.Source, ce.Controller)
		sc.PreparedSpell = e.preparedSpellOf(id)
		for _, r := range ce.Remembered {
			sc.Remembered = append(sc.Remembered, state.Target{Obj: r})
		}
		if e.matchesSpec(ce.RestrictParam(cards.PKValidCard), id, sc) {
			return true
		}
	}
	return false
}
