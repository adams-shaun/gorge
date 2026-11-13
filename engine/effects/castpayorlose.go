package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// CastPayOrLoseCost reports the mana a spell's caster must pay at the
// beginning of their next upkeep or lose the game (the Pact cycle:
// Summoner's Pact {2}{G}{G}, Slaughter Pact {2}{B}), read off the spell's
// ability chain: a DelayedTrigger on the caster's upkeep whose execute chain
// holds a LosesGame for the caster, paid by the caster, with a plain mana
// UnlessCost$. The result is the cost in Forge's spelling ("2 G G"); it is
// empty for every other face, for a cost that is not plain mana (a Sac or
// life payment, an X), and for a nil face. Never keyed on a card name.
func CastPayOrLoseCost(f *cards.Face) string {
	if f == nil {
		return ""
	}
	sa := f.SpellAbility()
	for n, depth := sa, 0; n != nil && depth < 16; n, depth = n.Sub, depth+1 {
		if n.API != "DelayedTrigger" || n.Params["Mode"] != "Phase" || n.Params["Phase"] != "Upkeep" {
			continue
		}
		if v := n.Params["ValidPlayer"]; v != "" && v != "You" {
			continue
		}
		exec := cards.ResolveSVar(f.SVars, n.Params["Execute"])
		for e, d := exec, 0; e != nil && d < 16; e, d = e.Sub, d+1 {
			if e.API != "LosesGame" {
				continue
			}
			if v := e.Params["Defined"]; v != "" && v != "You" {
				continue
			}
			if v := e.Params["UnlessPayer"]; v != "" && v != "You" {
				continue
			}
			if c := e.Params["UnlessCost"]; plainManaCost(c) {
				return c
			}
		}
	}
	return ""
}

// plainManaCost reports whether c is a non-empty cost of generic numbers
// and single colour letters only ("2 G G"), the spelling a Forge mana cost
// takes: no X, hybrid, Phyrexian or non-mana component.
func plainManaCost(c string) bool {
	toks := strings.Fields(c)
	if len(toks) == 0 {
		return false
	}
	for _, t := range toks {
		if len(t) == 1 && strings.Contains("WUBRGC", t) {
			continue
		}
		for i := 0; i < len(t); i++ {
			if t[i] < '0' || t[i] > '9' {
				return false
			}
		}
	}
	return true
}
