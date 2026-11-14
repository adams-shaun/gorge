package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// Cleave (CR 702.148a). Forge scripts it as a second A:SP$ line carrying
// NonBasicSpell$ True: its Cost$ is the cleave cost and its parameters are
// already the bracket-free text (gorge engine/rules/cleave.go cleaveSA). A
// cleaved cast pays that cost instead of the mana cost and resolves that
// line instead of the printed one.
//
// kgen lowers the second line as a latent spell ability of its own (its own
// targets and program: the text change moves a target filter, not just the
// verb) and records its cost: CardDef.CleaveAb names the ability and
// CardDef.CleaveCost is the alternative cost. The kernel's cleaved cast puts
// that ability index on the stack entry (dirac/kw_cleave.go).

// cleaveSAOf is f's cleave spell ability: the first SP line after the face's
// own that carries NonBasicSpell$ True, or nil (gorge cleaveSA's rule).
func cleaveSAOf(f *cards.Face) *cards.SA {
	if f == nil {
		return nil
	}
	first := true
	for _, a := range f.Abilities {
		if a == nil || a.Kind != "SP" {
			continue
		}
		if first {
			first = false
			continue
		}
		if a.Params["NonBasicSpell"] == "True" {
			return a
		}
	}
	return nil
}

// isCleaveSA reports whether sa is the face's cleave line.
func (lc *lowerCtx) isCleaveSA(sa *cards.SA) bool {
	return sa != nil && sa.Kind == "SP" && cleaveSAOf(lc.face) == sa
}

// refuseCleaveCombos refuses Cleave beside any other cast cost route: the
// kernel's cleaved cast pays one plain alternative mana cost of a
// single-face instant or sorcery (the refuseOverloadCombos shape).
func (lc *lowerCtx) refuseCleaveCombos() {
	out := lc.out
	if out.Cleave == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Dash != nil || out.Evoke != nil || out.MayFlash != nil || out.Delve || out.Storm || out.SpellWaterbend != 0 || out.BeholdN != 0 || out.BeholdExile || out.Miracle != nil ||
		out.Overload != nil || out.Surge != nil || out.Rebound != nil || out.Cost.X || out.Cost.XX || out.Cost.XXX {
		lc.refuseNode("A:SP$ NonBasicSpell", "Cleave beside an alternative or additional cost, or an X cost")
		return
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("A:SP$ NonBasicSpell", "Cleave on a multi-face card")
	}
}
