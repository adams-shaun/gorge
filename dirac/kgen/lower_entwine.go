package kgen

import "strings"

// K:Entwine (CR 702.42a): an optional additional cost on a modal spell; paid,
// the spell follows the instructions of all its modes (CR 702.42b). gorge
// offers it as its own cast option (Mode "entwined", the Buyback shape: base
// plus the entwine cost, rules legal_walk_hand.go) and announces every mode
// with a legal target (castModeAsk: min = max = that count). The kernel's
// kicked cast is that option: the cost rides CardDef.Kicker (no KwKicker; a
// Sac<N/..> cost is zero mana plus CardDef.KickerCost), CardDef.Entwine names
// it and the kicked entry's mode step picks every feasible mode
// (dirac/kw_entwine.go).

// lowerEntwine lowers the keyword parameter: a mana cost, or one Sac<N/..>
// part with no mana (Betrayal of Flesh's Sac<3/Land>). Anything else reports
// false and the caller refuses it by name.
func (lc *lowerCtx) lowerEntwine(param string) bool {
	if lc.out.Entwine != nil {
		lc.refuseNode("K:Entwine", "second Entwine keyword")
		return true
	}
	if c, err := lc.cost(param); err == nil && c.Mana.None && c.Sac != nil && c.SacN >= 1 {
		rest := c
		rest.Sac, rest.SacN = nil, 0
		if rest.pureMana() && rest.Mana.None {
			lc.out.Entwine, lc.out.KickerCost = &manaCost{None: true}, &c
			return true
		}
	}
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.Contains(param, ":") || strings.Contains(param, "<") || mc.X || mc.None {
		return false
	}
	lc.out.Entwine = &mc
	return true
}

// refuseEntwineCombos refuses Entwine on anything but a single-faced instant
// or sorcery whose spell ability is one plain "choose one" charm (gorge's
// entwined offer needs a Charm spell, and the all-modes announcement is the
// kernel's multi-pick step over the one-pick charm: a CharmNum$ beyond one, a
// MinCharmNum$, a repeat, a dynamic count or a mode restriction has no
// gorge-exact composition), and beside any other cast cost route: the kicked
// cast is one option with one extra cost.
func (lc *lowerCtx) refuseEntwineCombos() {
	out := lc.out
	if out.Entwine == nil {
		return
	}
	if out.Types&(tInstant|tSorcery) == 0 || out.Types&tPermanent != 0 {
		lc.refuseNode("K:Entwine", "Entwine on a permanent card")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Entwine", "Entwine on a multi-face card")
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Buyback != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Miracle != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree ||
		out.WebSling != nil || out.Evoke != nil || out.Dash != nil || out.MayFlash != nil || out.MayFlashBehold != nil || out.BeamMeUp != nil || out.Delve ||
		out.SpellWaterbend != 0 || out.BeholdN != 0 || out.Storm || out.Flashback != nil || out.Multikicker != nil {
		lc.refuseNode("K:Entwine", "Entwine beside another cast route or additional cost")
	}
	for _, k := range out.Kw {
		if head, _, _ := strings.Cut(k, ":"); twobridKw[head] && head != "Entwine" {
			lc.refuseNode("K:Entwine", "Entwine beside %s", head)
		}
	}
	modal := 0
	for _, ab := range out.Abilities {
		if ab == nil || ab.Modes == 0 {
			continue
		}
		modal++
		if ab.Kind != akSpell || ab.ModesPick > 1 || ab.ModesMin > 1 || ab.ModeRestrict != 0 || ab.RepBudget != 0 || ab.RepMax != [4]uint8{} {
			lc.refuseNode("K:Entwine", "Entwine on a charm that is not a plain choose-one spell")
		}
	}
	if modal != 1 || out.ModesPickV.Kind != vNone || out.ModesPickTW != 0 || out.ModesMinCount != 0 {
		lc.refuseNode("K:Entwine", "Entwine on a card without exactly one plain charm")
	}
}

// entwineTargets checks the target shapes of an Entwine charm, which the
// kernel may announce as a multi-pick charm (every feasible mode chosen). One
// targeted mode takes the ordinary ask, as gorge does (a lone slot, not
// askCharmModeTargets); two or more take the multi-pick rules
// (charmMultiTargets). A mode whose root declares no target but a chain link
// does (Grab the Reins' second mode) is gorge's chain link, announced after
// the roots: it is marked in Ability.ChainModes, and must follow every rooted
// mode, because the kernel asks target slots in slot order.
func (lc *lowerCtx) entwineTargets(nModes int, rootTargeted []bool) error {
	if nModes > 4 {
		return refuse("charm with more than 4 modes") // mode bits 0..3 only
	}
	targeted := make([]bool, nModes)
	for _, t := range lc.ab.Targets {
		if t.Mode < 0 || t.Mode >= nModes {
			return refuse("Entwine charm with a non-mode target")
		}
		targeted[t.Mode] = true
	}
	chainSeen, n := false, 0
	var chain uint8
	for m := 0; m < nModes; m++ {
		if !targeted[m] {
			continue
		}
		n++
		if rootTargeted[m] {
			if chainSeen {
				return refuse("Entwine charm mode with a root target after a chain-only mode")
			}
			continue
		}
		chainSeen = true
		chain |= 1 << m
	}
	if n < 2 {
		if chain != 0 {
			return refuse("Entwine charm mode whose root declares no target")
		}
		return nil
	}
	over := append([]bool(nil), rootTargeted...)
	for m := range over {
		over[m] = over[m] || chain&(1<<m) != 0
	}
	if err := lc.charmMultiTargets(nModes, over); err != nil {
		return err
	}
	lc.ab.ChainModes = chain
	return nil
}
