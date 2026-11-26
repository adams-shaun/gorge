package kgen

import "strings"

func init() { kwHandlers["Blitz"] = (*lowerCtx).lowerBlitzKw }

// lowerBlitzKw lowers a printed K:Blitz:<mana cost> (CR 702.152a): the
// printed keyword is CardDef.HasBlitz with its cost in BlitzCost
// (kop.KwBlitz). The kernel offers the blitzed cast at that cost from the
// hand (gorge's "blitzed" option, which its planner does not price, so only a
// floating pool that pays offers it), and the permanent that cast produces
// gains haste until end of turn, "when this creature dies, draw a card" and a
// sacrifice at the beginning of the next end step (dirac/kw_blitz.go).
// Gorge reads the printed line directly (engine/rules/blitz.go), so no
// expansion line belongs to the head. A cost with a non-mana part (Discard<>,
// PayLife<>, Sac<>), an {X} or CardManaCost cost, the ":" filter suffix of a
// granted instance, and a second K:Blitz line refuse; so does a GRANTED Blitz
// (a static's AddKeyword$) and the graveyard cast of Mode$ Continuous
// ValidSA$ Spell.Blitz, which stay on their own lines.
func (lc *lowerCtx) lowerBlitzKw(node, param string) {
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None {
		lc.refuseNode(node, "blitz cost %q", param)
		return
	}
	kc, err := manaCostOf(mc)
	if err != nil {
		lc.refuseNode(node, "blitz cost %q: %v", param, err)
		return
	}
	if lc.out.Blitz {
		lc.refuseNode(node, "two Blitz keywords")
		return
	}
	lc.out.Blitz, lc.out.BlitzCost = true, kc
}

// refuseBlitzCombos refuses blitz beside any other cast cost route (the
// blitzed provenance shares stackEntry.mode's impending bit, which the
// kernel's alt-cost chain also sets for the cleaved, surged and overloaded
// casts, dirac/cast.go; blitzed() reads HasBlitz && impended(), so a printed
// Blitz beside those would run the blitz riders on their cast), on an {X}
// spell or on a multi-face card, the refuseDashCombos shape.
func (lc *lowerCtx) refuseBlitzCombos() {
	out := lc.out
	if !out.Blitz {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Evoke != nil || out.Dash != nil || out.MayFlash != nil || out.Overload != nil || out.Surge != nil || out.Cleave != nil {
		lc.refuseNode("K:Blitz", "Blitz beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Blitz", "Blitz on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Blitz", "Blitz on a multi-face card")
	}
}
