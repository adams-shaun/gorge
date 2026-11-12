package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// graveyardAltCostCasts appends the graveyard casts a card carries on itself
// through S:Mode$ AlternativeCost | EffectZone$ Graveyard (Worldheart Phoenix,
// Raffine's Guidance, Squee, Dubious Monarch, Scourge of Nel Toth). The hand
// walk already offers such an alternative from hand at the same
// AltCostIndex; only the explicit-Graveyard statics are offered here, never
// an EffectZone$ All one (Verdant Mastery, Bringer of the Red Dawn) and never
// a battlefield grant that happens to match a graveyard card.
//
// The cost is not charged here: beginCast resolves opt.AltCostIndex against
// e.alternativeCosts(p, id)[AltCostIndex-1], the FULL list, exactly as it
// does for the hand offer -- so i is the list index, not a filtered one.
func (w *legalWalk) graveyardAltCostCasts(grave []state.ObjID) {
	e, p := w.e, w.p
	sorcery := w.sorcery
	out := &w.out
	for _, id := range grave {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZGraveyard {
			continue
		}
		f := o.Face()
		// The self-carried static is the only source of a graveyardSelf
		// entry, so this cheap face probe skips the whole alternativeCosts
		// build for every other graveyard card.
		if !faceHasGraveyardSelfAltCost(f) {
			continue
		}
		for i, alt := range e.alternativeCosts(p, id) {
			if !alt.graveyardSelf || alt.src != id {
				// A battlefield or Effect-delivered grant can match a card
				// in the graveyard, but it is not this permission. Advance i
				// anyway: the index is into the full list.
				continue
			}
			// The spell's own additional costs are paid beside the
			// alternative cost (CR 118.9d), as the hand arm composes them.
			gate := pay.WithSpellAbilityExtras(f, alt.cost)
			altScope := spellScope("")
			if alt.manaRestriction != "" {
				if !pay.OnlyTagKnown(alt.manaRestriction) {
					continue
				}
				altScope.OnlyTag = alt.manaRestriction
			}
			if alt.announce != "" {
				gate.Exile = nil
				if len(e.altCostXCandidates(p, id, alt)) == 0 {
					continue
				}
			}
			if e.castSuppressed(p, id) || w.castRestrictedCost(p, id, gate) ||
				!w.spellTimingOK(p, id, f, sorcery) ||
				!e.castTargetsAvailable(p, id, f.SpellAbility()) {
				continue
			}
			if w.altAddOfferCastable(p, id, f, gate, altScope) {
				*out = append(*out, decision.Option{Index: len(*out), Kind: "cast",
					Label: altCostLabel(f.Name, i), Obj: id, AltCostIndex: i + 1})
			}
		}
	}
}
