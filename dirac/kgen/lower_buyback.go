package kgen

import "strings"

// K:Buyback (CR 702.27a): an optional additional cost; the spell returns to
// its owner's hand as it resolves (dirac/kw_buyback.go). The kernel's kicked
// cast is gorge's "(buyback)" cast option, the Offspring shape: the cost
// rides CardDef.Kicker (no KwKicker), the paid flag is the entry's sfKicked.

// lowerBuyback lowers the keyword parameter: a mana cost, or one Sac<1/..>
// or Discard<N/Card> part (Kicker zero mana plus KickerCost,
// lower_kickercost.go; the discard is the ordinary discard cost pick, which
// never takes the spell itself). Anything else (PayLife<..>, a Sac<N/..> of
// several, a random or filtered discard) reports false and the caller
// refuses it by name.
func (lc *lowerCtx) lowerBuyback(param string) bool {
	if lc.out.Buyback != nil {
		lc.refuseNode("K:Buyback", "second Buyback keyword")
		return true
	}
	if kc, ok := lc.kickerNonMana(param); ok && kc.Sac != nil {
		lc.out.Buyback, lc.out.KickerCost = &manaCost{None: true}, kc
		return true
	}
	if c, err := lc.cost(param); err == nil && c.Mana.None && c.DiscardN >= 1 && c.DiscardN <= 4 && c.DiscardSpec == "" && c.DiscardF == nil {
		rest := c
		rest.DiscardN = 0
		if rest.pureMana() && rest.Mana.None {
			lc.out.Buyback, lc.out.KickerCost = &manaCost{None: true}, &c
			return true
		}
	}
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.Contains(param, ":") || strings.Contains(param, "<") || mc.X || mc.None {
		return false
	}
	lc.out.Buyback = &mc
	return true
}

// refuseBuybackCombos refuses Buyback on anything but a single-faced instant
// or sorcery (CR 702.27a is a spell's rule; the return is the resolving
// spell's), and beside any other cast cost route: the kicked cast is one
// option with one extra cost, so a second extra cost, an alternative cost or
// a cost-paying keyword has no free bit and no gorge-exact composition.
func (lc *lowerCtx) refuseBuybackCombos() {
	out := lc.out
	if out.Buyback == nil {
		return
	}
	if out.Types&(tInstant|tSorcery) == 0 || out.Types&tPermanent != 0 {
		lc.refuseNode("K:Buyback", "Buyback on a permanent card")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Buyback", "Buyback on a multi-face card")
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Miracle != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree ||
		out.WebSling != nil || out.Evoke != nil || out.Dash != nil || out.MayFlash != nil || out.MayFlashBehold != nil || out.BeamMeUp != nil || out.Delve ||
		out.SpellWaterbend != 0 || out.BeholdN != 0 || out.Storm {
		lc.refuseNode("K:Buyback", "Buyback beside another cast route or additional cost")
	}
	for _, k := range out.Kw {
		if head, _, _ := strings.Cut(k, ":"); twobridKw[head] && head != "Buyback" {
			lc.refuseNode("K:Buyback", "Buyback beside %s", head)
		}
	}
}
