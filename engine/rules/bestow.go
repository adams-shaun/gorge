package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
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
//     (engine/rules/cast.go) and at resolution (engine/rules/stack.go), keyed on the
//     pendingCast mode and the pay-time FlagBestowed provenance — never
//     added to the face itself, which would make the PLAIN creature cast
//     target-bearing and route its resolution to Attach.
//   - the bestowed type switch (CR 702.114e) is derived from live state on
//     state.Object.BestowedAttached and consulted at the filter grammar
//     (engine/effects/filter.go hasType), the layer-4 type derivation
//     (engine/rules/layers.go typeCharacteristics), the combat/SBA creature reads
//     and the attachment SBAs (engine/rules/attach.go).

// bestowCost resolves the Bestow keyword's alternative cost (CR 702.114a,
// Forge's K:Bestow:<cost>), the replicateCost shape: the cost is paid
// INSTEAD of the printed mana cost. A cost carrying a token ParseCost
// cannot model is withheld rather than charged as degraded generic mana (the
// replicate fail-closed convention).
//
// Two shapes are priced rather than withheld:
//
//   - An {X} cost (nyxborn_hydra's "X G G") announces X through the ordinary
//     cast-time X machinery, exactly as a printed {X} mana cost does
//     (CR 601.2b): bestowCost no longer withholds on Cost.X.
//   - CollectEvidence<N> (detectives_phoenix's "R CollectEvidence<6>") is a
//     real modelled cost part (engine/rules/mana.go's evidenceCost), settled by the
//     shared CollectEvidence payment stage, so ParseCost no longer reports it
//     as Unknown.
//
// The keyword parameter is occasionally followed by Forge's trailing fields;
// only the first colon-free field is the cost (hypnotic_siren's
// "5 U U:GainControl", where ":GainControl" is Forge metadata for the card's
// own S:Mode$ Continuous GainControl$ static, not cost text). This is the
// same colon cut mutateCost practices.
func bestowCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Bestow")
	if !ok {
		return Cost{}, false
	}
	cost, _, _ := strings.Cut(s, ":")
	c := ParseCost(strings.TrimSpace(cost))
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// bestowedAttachSA is the attach spell-ability a bestowed cast resolves
// (CR 702.114a: "cast it for its bestow cost... it's an Aura spell with
// enchant creature"): the same SP$ Attach shape the K:Enchant expansion
// (engine/cards/keywords.go's case "Enchant") writes for an ordinary Aura. Bestow
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

// bestowTargetIllegal is CR 702.103e's resolution-start check for a
// bestowed Aura spell: true when none of its targets is still legal for the
// synthesized enchant-creature attach SA, revalidated with the same
// legalTargets helper resolveTop's CR 608.2b recheck uses for every spell.
func (e *Engine) bestowTargetIllegal(o *state.Object, sa *cards.SA) bool {
	return len(e.legalTargets(o.Targets, sa, targetZones(sa), o.Controller, o.ID, o.ID)) == 0
}

func init() {
	// Coverage: the keyword head engine/cards/primitive.go derives for every
	// K:Bestow line is now engine-supported (the bestowed cast, the attached
	// type switch and the detach transition all live in rules).
	effects.RegisterNonAPI("kw:Bestow")
}
