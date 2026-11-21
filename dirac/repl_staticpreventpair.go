package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The static damage prevention (repl_staticprevent.go) against the damage
// replacements it does not commute with.

// usePlusAbility: the table has a ReplDamagePlus line (its card carries
// rbPlusDmg, which every static prevention host carries too).
var usePlusAbility = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDamagePlus {
				return true
			}
		}
	}
	return false
})

// usePrevColOpen: the table has an Ext damage prevention record the static
// prevention's walk does not pose beside itself: a colour, a depleting
// PreventNext shield, or a scoped prevention that can match noncombat damage
// or carries a rider. A combat-only scoped prevention (Arachnogenesis, Loyal
// Unicorn) is a full prevention record the combat walk collects beside the
// static prevention (replOrderCandsFz: prevColCands, then spCombatCands); it
// is the one record kind the pair is lifted for.
var usePrevColOpen = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if prevRecOpen(in) {
					return true
				}
			}
		}
	}
	return false
})

// prevRecOpen: in is an Ext damage prevention record other than a combat-only
// scoped full prevention without a rider.
func prevRecOpen(in kop.Instr) bool {
	switch in.Ext() {
	case kop.ExtPreventColor, kop.ExtPreventNext:
		return true
	case kop.ExtPreventDamageScope:
		return in.C&kop.PrevScopeCombat == 0 || in.C&(kop.PrevScopeGain|kop.PrevScopeEndCombat) != 0
	}
	return false
}

// spCompete are the damage replacement bits that can match the same event as a
// static prevention.
const spCompete = rbDblDmg | rbPrevNC | rbPrevDmgTo | rbPrevCol | rbShield | rbPrevCombatTo | rbPrevCombatBy | rbPrevMill | rbSelfDmg

// staticPrevStall: the table has a static damage prevention beside another
// damage replacement. gorge asks the CR 616.1 order of the two and the
// amounts differ with it, so initStall refuses the pair.
func staticPrevStall(g *genPrep) bool {
	return g.uses.has(useStaticPrev) && (g.anyRepl&spCompete&^rbPrevCol != 0 || g.anyRepl&rbPrevCol != 0 && g.uses.has(usePrevColOpen) ||
		g.anyRedir || g.uses.has(usePlusAbility))
}

// staticPrevPair is pairRefusal's halves for staticPrevStall: the static
// prevention hosts, and the cards carrying another damage replacement (a
// doubler, plus-N, prevention, self-damage replacement, a redirect, a
// prevention spell or a SHIELD counter placer).
func staticPrevPair(g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	text := "a static damage prevention (Prevent$ True or a prevent-N shield) beside another damage replacement: gorge asks the CR 616.1 order of the two, and the damage left depends on it"
	a := abilityOn(isStaticPrev)
	b := func(c *kop.CardDef, i int) bool {
		return g.replBits[i]&spCompete != 0 || abilityOn(func(x *kop.Ability) bool {
			return x.Kind == kop.AbReplacement && (x.Repl.Kind == kop.ReplDamagePlus || x.Repl.Kind == kop.ReplEntersWithCounters && x.Repl.C == int16(kop.CtrShield)) ||
				x.Cost.Flags&kop.CostAddCtr != 0 && x.Cost.Ctr == kop.CtrShield ||
				progHas(x, func(in *kop.Instr) bool {
					e := in.Ext()
					return prevRecOpen(*in) || e == kop.ExtDmgRedirect ||
						(in.Op == kop.OpPutCounter || in.Op == kop.OpPutCounterAll || in.Op == kop.OpEnterWithCounters) && in.C == int16(kop.CtrShield)
				})
		})(c, i)
	}
	return text, a, b
}
