package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StallDiamondMix: a table holding a Lion's Eye Diamond (gfDiscHand) beside
// a mana source of another census-gap kind (an each-colour source, a filter,
// a restricted or any-colour restricted source, a deferred or Taps-deferred
// source, a counter-cost or tap-another source, a granted mana ability).
// gorge's one prefix search activates every census gap together
// (potential_script.go scriptSearch.visit); the kernel models the Diamond
// only when it is its controller's only gap kind (mana_discardhand.go
// dhBoard), so such a pair is refused at its start. The check is table-wide:
// a Diamond in one deck and a gap source in the other is refused too.
const StallDiamondMix = 101

func init() { stallNames[StallDiamondMix] = "diamondmix" }

// prepDiamondMix sets genPrep.dhMix when t holds a Diamond and a source of
// another census-gap kind: StallDiamondMix.
func prepDiamondMix(t *kop.Table, g *genPrep, p *prep) {
	if !g.anyDiscHand {
		return
	}
	g.dhMix = g.anyScriptGap || g.anyDeferred || g.anyTapsDefer || len(g.grantPC) != 0 || p.ctrMana
	for i := range t.Cards {
		g.dhMix = g.dhMix || dhMixGap(t, g, p, i)
	}
}

// dhMixGap reports whether card i carries a census-gap kind other than the
// Diamond's own: every mask dhBoard tests besides dhGaps.
func dhMixGap(t *kop.Table, g *genPrep, p *prep, i int) bool {
	c := CardID(i)
	if g.has(c, gfEachCol|gfRestrScript|gfDeferred|gfAnyR|gfFilt|gfTapsDefer) {
		return true
	}
	cd := &t.Cards[i]
	if g.has(c, gfGapOther) && !kop.DiscardHandManaAb(&cd.Abilities[g.gapOther(c)]) {
		return true // (the Diamond's own ability is a gapOther: it does not count itself)
	}
	if p.tapXAb[i] >= 0 || p.ctrAb[i] >= 0 {
		return true // a tap-another or counter-cost source (mana_tapanother.go, mana_countercost.go)
	}
	return abilityOn(func(a *kop.Ability) bool {
		if a.Kind != kop.AbMana {
			return false
		}
		// a planner-deferred rider source (mana_rider.go) or a granted mana
		// ability (grantPC)
		_, _, rider := manaCtrRider(a)
		return rider || a.AFlags&kop.AfLatent != 0
	})(cd, i)
}
