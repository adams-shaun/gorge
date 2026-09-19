package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
)

// Bestow (CR 702.114) — the alternative-cost cast of a card printed with
// K:Bestow. The three pieces live here:
//
//   - bestowCost resolves the printed bestow cost (the replicateCost
//     convention: an unpriceable token withholds the offer, never charges a
//     degraded generic).
//   - bestowedAttachSA is the Aura spell-ability a bestowed cast resolves
//     with, hand-built because the creature face carries no SP of its own.
//     It is substituted for f.SpellAbility() at the target ask
//     (rules/cast.go) and at resolution (rules/stack.go), keyed on the
//     pendingCast mode and the pay-time FlagBestowed provenance — never
//     added to the face itself, which would make the PLAIN creature cast
//     target-bearing and route its resolution to Attach.
//   - the bestowed type switch (CR 702.114e) is derived from live state on
//     state.Object.BestowedAttached and consulted at the filter grammar
//     (effects/filter.go hasType), the layer-4 type derivation
//     (rules/layers.go typeCharacteristics), the combat/SBA creature reads
//     and the attachment SBAs (rules/attach.go).

// bestowCost resolves the Bestow keyword's alternative cost (CR 702.114a,
// Forge's K:Bestow:<cost>), the replicateCost shape: the cost is paid
// INSTEAD of the printed mana cost. A cost carrying a token ParseCost
// cannot model is withheld rather than charged as degraded generic mana --
// and so is one carrying {X} (nyxborn_hydra's "X G G"): the announced-X
// machinery exists, but no bestowed-X cast is proven end to end, so the
// offer stays withheld per the replicate fail-closed convention. The three
// exotic carriers (nyxborn_hydra, detectives_phoenix, hypnotic_siren --
// whose ":GainControl" suffix is Forge metadata for its unregistered
// GainControl static, not cost text) therefore never offer the bestowed
// cast.
func bestowCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Bestow")
	if !ok {
		return Cost{}, false
	}
	c := ParseCost(s)
	if len(c.Unknown) > 0 || c.X > 0 {
		return Cost{}, false
	}
	return c, true
}

// bestowedAttachSA is the attach spell-ability a bestowed cast resolves
// (CR 702.114a: "cast it for its bestow cost... it's an Aura spell with
// enchant creature"): the same SP$ Attach shape the K:Enchant expansion
// (cards/keywords.go's case "Enchant") writes for an ordinary Aura. Bestow
// is always "enchant creature" (measured over all 43 corpus carriers), so
// the spec is the literal Creature.
func bestowedAttachSA() *cards.SA {
	return &cards.SA{Kind: "SP", API: "Attach", Params: map[string]string{
		"ValidTgts": "Creature",
		"TgtPrompt": "Select target creature",
		"Object":    "Self",
		"Keyword":   "Bestow",
	}}
}

func init() {
	// Coverage: the keyword head cards/primitive.go derives for every
	// K:Bestow line is now engine-supported (the bestowed cast, the attached
	// type switch and the detach transition all live in rules).
	effects.RegisterNonAPI("kw:Bestow")
}
